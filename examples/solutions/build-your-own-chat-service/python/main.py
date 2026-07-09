"""Reference chat service gateway (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import threading

DELIVERY_QUEUE = "delivery"


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
            chunk = conn.recv(4096)
            if not chunk:
                raise RuntimeError(f"no response from {addr}")
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise RPCError(e["code"], e["message"])
        return resp.get("result", {})
    finally:
        conn.close()


class Engine:
    def __init__(self):
        self.lock = threading.Lock()
        self.ready = False
        self.idgen = os.environ.get("IDGEN_ADDR", "")
        self.log = os.environ.get("LOG_ADDR", "")
        self.queue = os.environ.get("QUEUE_ADDR", "")

    def ensure_stack(self):
        with self.lock:
            if self.ready:
                return
            if not self.idgen or not self.log or not self.queue:
                raise GWError("INTERNAL", "missing IDGEN_ADDR, LOG_ADDR, or QUEUE_ADDR")
            rpc(self.idgen, "configure", {"worker_id": 1})
            self.ready = True

    def handle(self, method, params):
        if method == "ping":
            self.ensure_stack()
            return {"message": "pong"}
        self.ensure_stack()
        if method == "send_message":
            conv = params.get("conversation_id")
            sender = params.get("sender")
            body = params.get("body")
            if not conv or not sender or not body:
                raise GWError("INVALID_PARAMS", "send_message requires conversation_id, sender, body")
            id_res = rpc(self.idgen, "next_id", {})
            env = json.dumps({
                "message_id": id_res["id"],
                "conversation_id": conv,
                "sender": sender,
                "body": body,
            })
            log_res = rpc(self.log, "append", {"topic": conv, "value": env})
            rpc(self.queue, "send", {"queue": DELIVERY_QUEUE, "body": env})
            return {"message_id": id_res["id"], "offset": log_res["offset"]}
        if method == "read_messages":
            conv = params.get("conversation_id")
            if not conv:
                raise GWError("INVALID_PARAMS", "read_messages requires conversation_id")
            offset = params.get("offset", 0)
            mx = params.get("max", 100)
            log_res = rpc(self.log, "read", {"topic": conv, "offset": offset, "max": mx})
            return {"records": log_res.get("records", [])}
        if method == "poll_delivery":
            q_res = rpc(self.queue, "receive", {"queue": DELIVERY_QUEUE, "visibility_timeout_ms": 30000})
            msg = q_res.get("message")
            if not msg:
                return {"message": None}
            env = json.loads(msg["body"])
            return {
                "message": {
                    "message_id": env["message_id"],
                    "conversation_id": env["conversation_id"],
                    "sender": env["sender"],
                    "body": env["body"],
                    "receipt": msg["receipt"],
                }
            }
        if method == "ack_delivery":
            receipt = params.get("receipt")
            if not receipt:
                raise GWError("INVALID_PARAMS", "ack_delivery requires receipt")
            rpc(self.queue, "ack", {"receipt": receipt})
            return {}
        raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


ENGINE = Engine()


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            line = line.decode().strip()
            if not line:
                continue
            req = json.loads(line)
            try:
                result = ENGINE.handle(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except GWError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            except Exception as e:
                resp = {"id": req.get("id"), "error": {"code": "INTERNAL", "message": str(e)}}
            self.wfile.write((json.dumps(resp) + "\n").encode())
            self.wfile.flush()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--data-dir", default="")
    args = p.parse_args()
    srv = socketserver.ThreadingTCPServer(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}")
    srv.serve_forever()


if __name__ == "__main__":
    main()
