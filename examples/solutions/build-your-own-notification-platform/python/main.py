"""Reference notification platform gateway (Python). Passes all 9 stages."""

import argparse
import itertools
import json
import os
import socket
import socketserver
import threading
import time

QUEUE_NAME = "notifications"
_seq = itertools.count(1)
_seq_lock = threading.Lock()


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


def next_id():
    with _seq_lock:
        return f"n-{next(_seq)}"


class Engine:
    def __init__(self):
        self.queue = os.environ.get("QUEUE_ADDR", "")
        self.scheduler = os.environ.get("SCHEDULER_ADDR", "")
        self.ratelimiter = os.environ.get("RATE_LIMITER_ADDR", "")

    def dispatch_due(self):
        while True:
            pr = rpc(self.scheduler, "poll", {})
            job = pr.get("job")
            if not job:
                break
            body = json.dumps(job["payload"])
            rpc(self.queue, "send", {"queue": QUEUE_NAME, "body": body})
            rpc(self.scheduler, "complete", {"lease_token": job["lease_token"]})

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "configure_limit":
            user_id = params.get("user_id")
            limit = params.get("limit")
            window_ms = params.get("window_ms")
            if not user_id or not limit or not window_ms:
                raise GWError("INVALID_PARAMS", "configure_limit requires user_id, limit, window_ms")
            rpc(
                self.ratelimiter,
                "configure",
                {
                    "key": f"user:{user_id}",
                    "algorithm": "fixed_window",
                    "limit": limit,
                    "window_ms": window_ms,
                },
            )
            return {}
        if method == "notify":
            user_id = params.get("user_id")
            channel = params.get("channel")
            body = params.get("body")
            if not user_id or not channel:
                raise GWError("INVALID_PARAMS", "notify requires user_id, channel, body")
            try:
                take = rpc(self.ratelimiter, "take", {"key": f"user:{user_id}", "cost": 1})
            except GWError as e:
                if e.code == "KEY_NOT_FOUND":
                    raise GWError("INVALID_PARAMS", "limit not configured for user") from e
                raise
            if not take.get("allowed"):
                return {"notification_id": None, "queued": False, "rate_limited": True}
            nid = next_id()
            env = {"notification_id": nid, "user_id": user_id, "channel": channel, "body": body}
            rpc(self.queue, "send", {"queue": QUEUE_NAME, "body": json.dumps(env)})
            return {"notification_id": nid, "queued": True}
        if method == "schedule_notify":
            user_id = params.get("user_id")
            channel = params.get("channel")
            body = params.get("body")
            delay_ms = params.get("delay_ms")
            if not user_id or not channel or delay_ms is None:
                raise GWError("INVALID_PARAMS", "schedule_notify requires user_id, channel, body, delay_ms")
            nid = next_id()
            env = {"notification_id": nid, "user_id": user_id, "channel": channel, "body": body}
            res = rpc(self.scheduler, "schedule", {"payload": env, "delay_ms": delay_ms})
            return {"notification_id": nid, "job_id": res["job_id"]}
        if method == "receive":
            self.dispatch_due()
            recv = rpc(self.queue, "receive", {"queue": QUEUE_NAME, "visibility_timeout_ms": 30000})
            msg = recv.get("message")
            if not msg:
                return {"notification": None}
            env = json.loads(msg["body"])
            return {
                "notification": {
                    "notification_id": env["notification_id"],
                    "user_id": env["user_id"],
                    "channel": env["channel"],
                    "body": env["body"],
                    "receipt": msg["receipt"],
                }
            }
        if method == "ack":
            receipt = params.get("receipt")
            if not receipt:
                raise GWError("INVALID_PARAMS", "ack requires receipt")
            rpc(self.queue, "ack", {"receipt": receipt})
            return {}
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
