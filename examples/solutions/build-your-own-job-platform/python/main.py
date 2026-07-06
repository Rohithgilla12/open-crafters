"""Reference job platform gateway (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import threading
import time

QUEUE_NAME = "jobs"
LOCK_NAME = "dispatcher"
HOLDER_ID = "gateway"


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def rpc(addr, method, params):
    host, port_s = addr.rsplit(":", 1)
    conn = socket.create_connection((host, int(port_s)), timeout=10)
    try:
        rid = str(time.time_ns())
        conn.sendall((json.dumps({"id": rid, "method": method, "params": params}) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = conn.recv(4096)
            if not chunk:
                break
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise GWError(e["code"], e["message"])
        return resp.get("result", {})
    finally:
        conn.close()


class Engine:
    def __init__(self):
        self.scheduler = os.environ.get("SCHEDULER_ADDR", "")
        self.queue = os.environ.get("QUEUE_ADDR", "")
        self.lock = os.environ.get("LOCK_ADDR", "")

    def dispatch_due(self):
        try:
            tr = rpc(self.lock, "try_acquire", {"name": LOCK_NAME, "holder_id": HOLDER_ID, "lease_ms": 3000})
        except GWError:
            return
        if not tr.get("acquired"):
            return
        token = tr.get("token", "")
        try:
            while True:
                pr = rpc(self.scheduler, "poll", {})
                job = pr.get("job")
                if not job:
                    break
                body = json.dumps({
                    "job_id": job["job_id"],
                    "payload": job["payload"],
                    "lease_token": job["lease_token"],
                })
                rpc(self.queue, "send", {"queue": QUEUE_NAME, "body": body})
        finally:
            if token:
                rpc(self.lock, "release", {"name": LOCK_NAME, "token": token})

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "submit_job":
            if "payload" not in params or "delay_ms" not in params:
                raise GWError("INVALID_PARAMS", "submit_job requires payload and delay_ms")
            res = rpc(self.scheduler, "schedule", {"payload": params["payload"], "delay_ms": params["delay_ms"]})
            return {"job_id": res["job_id"]}
        if method == "receive_work":
            self.dispatch_due()
            recv = rpc(self.queue, "receive", {"queue": QUEUE_NAME, "visibility_timeout_ms": 30000})
            msg = recv.get("message")
            if not msg:
                return {"work": None}
            body = json.loads(msg["body"])
            return {
                "work": {
                    "job_id": body["job_id"],
                    "payload": body["payload"],
                    "lease_token": body["lease_token"],
                    "receipt": msg["receipt"],
                }
            }
        if method == "complete_work":
            lt = params.get("lease_token")
            rc = params.get("receipt")
            if not lt or not rc:
                raise GWError("INVALID_PARAMS", "complete_work requires lease_token and receipt")
            rpc(self.scheduler, "complete", {"lease_token": lt})
            rpc(self.queue, "ack", {"receipt": rc})
            return {}
        if method == "cancel_job":
            jid = params.get("job_id")
            if not jid:
                raise GWError("INVALID_PARAMS", "cancel_job requires job_id")
            res = rpc(self.scheduler, "cancel", {"job_id": jid})
            return {"cancelled": res.get("cancelled", False)}
        if method == "get_job":
            jid = params.get("job_id")
            if not jid:
                raise GWError("INVALID_PARAMS", "get_job requires job_id")
            res = rpc(self.scheduler, "get_job", {"job_id": jid})
            return {"status": res["status"]}
        raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


ENGINE = Engine()


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
            except Exception as e:
                resp = {"id": req.get("id"), "error": {"code": "INTERNAL", "message": str(e)}}
            self.wfile.write(json.dumps(resp).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--data-dir", default="")
    args = p.parse_args()
    with Server(("127.0.0.1", args.port), Handler) as srv:
        print(f"listening on 127.0.0.1:{args.port}", flush=True)
        srv.serve_forever()


if __name__ == "__main__":
    main()
