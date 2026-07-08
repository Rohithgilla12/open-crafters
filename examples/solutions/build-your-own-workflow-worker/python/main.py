"""Reference gateway for build-your-own-workflow-worker (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import time

DEFAULT_QUEUE = "default"
POLL_INTERVAL = 0.05
DRIVE_TIMEOUT = 30.0


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def rpc(addr, method, params):
    host, port_s = addr.rsplit(":", 1)
    conn = socket.create_connection((host, int(port_s)), timeout=10)
    try:
        conn.sendall((json.dumps({"id": "1", "method": method, "params": params}) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = conn.recv(65536)
            if not chunk:
                raise OSError(f"no response from {addr}")
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise RPCError(e["code"], e["message"])
        return resp.get("result")
    finally:
        conn.close()


def activity_result(activity_type, _input):
    if activity_type == "fetch":
        return {"status": 200, "body": "ok"}
    if activity_type == "work":
        return {"done": True}
    return {"ok": True}


class Engine:
    def __init__(self, temporal, sdk):
        self.temporal = temporal
        self.sdk = sdk

    def describe(self, workflow_id):
        res = rpc(self.temporal, "describe_workflow", {"workflow_id": workflow_id})
        return res["status"], res.get("result"), res.get("error")

    def replay(self, workflow_type, history):
        res = rpc(self.sdk, "replay", {"workflow_type": workflow_type, "history": history})
        return res.get("commands") or []

    def drive_round(self):
        for _ in range(8):
            wt_res = rpc(self.temporal, "poll_workflow_task", {"task_queue": DEFAULT_QUEUE})
            task = wt_res.get("task")
            if not task:
                break
            cmds = self.replay(task["workflow_type"], task["history"])
            rpc(
                self.temporal,
                "complete_workflow_task",
                {"task_token": task["task_token"], "commands": cmds},
            )
        for _ in range(8):
            at_res = rpc(self.temporal, "poll_activity_task", {"task_queue": DEFAULT_QUEUE})
            task = at_res.get("task")
            if not task:
                break
            result = activity_result(task["activity_type"], task.get("input"))
            rpc(
                self.temporal,
                "complete_activity_task",
                {"task_token": task["task_token"], "result": result},
            )

    def drive_until_done(self, workflow_id):
        deadline = time.monotonic() + DRIVE_TIMEOUT
        while time.monotonic() < deadline:
            status, result, err_val = self.describe(workflow_id)
            if status == "COMPLETED":
                return status, result, None
            if status == "FAILED":
                return status, None, err_val
            self.drive_round()
            time.sleep(POLL_INTERVAL)
        raise TimeoutError(f'workflow "{workflow_id}" did not finish within timeout')

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "start_workflow":
            wf_id = params.get("workflow_id")
            wf_type = params.get("workflow_type")
            if not wf_id or not wf_type:
                raise GWError("INVALID_PARAMS", "start_workflow requires workflow_id and workflow_type")
            q = params.get("task_queue") or DEFAULT_QUEUE
            try:
                res = rpc(
                    self.temporal,
                    "start_workflow",
                    {
                        "workflow_id": wf_id,
                        "workflow_type": wf_type,
                        "input": params.get("input"),
                        "task_queue": q,
                    },
                )
            except RPCError as e:
                raise GWError(e.code, e.message) from e
            return {"run_id": res["run_id"]}
        if method == "signal_workflow":
            wf_id = params.get("workflow_id")
            sig = params.get("signal_name")
            if not wf_id or not sig:
                raise GWError("INVALID_PARAMS", "signal_workflow requires workflow_id and signal_name")
            try:
                rpc(
                    self.temporal,
                    "signal_workflow",
                    {
                        "workflow_id": wf_id,
                        "signal_name": sig,
                        "input": params.get("input"),
                    },
                )
            except RPCError as e:
                raise GWError(e.code, e.message) from e
            return {}
        if method == "await_workflow":
            wf_id = params.get("workflow_id")
            if not wf_id:
                raise GWError("INVALID_PARAMS", "await_workflow requires workflow_id")
            status, result, err_val = self.drive_until_done(wf_id)
            return {"status": status, "result": result, "error": err_val}
        if method == "run_workflow":
            wf_id = params.get("workflow_id")
            wf_type = params.get("workflow_type")
            if not wf_id or not wf_type:
                raise GWError("INVALID_PARAMS", "run_workflow requires workflow_id and workflow_type")
            q = params.get("task_queue") or DEFAULT_QUEUE
            try:
                rpc(
                    self.temporal,
                    "start_workflow",
                    {
                        "workflow_id": wf_id,
                        "workflow_type": wf_type,
                        "input": params.get("input"),
                        "task_queue": q,
                    },
                )
            except RPCError as e:
                raise GWError(e.code, e.message) from e
            status, result, err_val = self.drive_until_done(wf_id)
            return {"status": status, "result": result, "error": err_val}
        raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


def make_engine():
    temporal = os.environ.get("TEMPORAL_ADDR", "")
    sdk = os.environ.get("SDK_ADDR", "")
    if not temporal or not sdk:
        raise SystemExit("TEMPORAL_ADDR and SDK_ADDR must be set by the harness")
    return Engine(temporal, sdk)


ENGINE = None


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            try:
                result = ENGINE.handle(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except GWError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            except RPCError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            except Exception as e:
                resp = {"id": req.get("id"), "error": {"code": "INTERNAL", "message": str(e)}}
            self.wfile.write(json.dumps(resp).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    global ENGINE
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--data-dir", default="")
    args = p.parse_args()
    ENGINE = make_engine()
    with Server(("127.0.0.1", args.port), Handler) as srv:
        print(f"workflow worker gateway listening on 127.0.0.1:{args.port}", flush=True)
        srv.serve_forever()


if __name__ == "__main__":
    main()
