"""Reference URL shortener gateway (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import threading
import time


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def rpc(addr, method, params):
    conn = socket.create_connection((addr.split(":")[0], int(addr.split(":")[1])), timeout=10)
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
        self.lock = threading.Lock()
        self.ready = False
        self.idgen = os.environ.get("IDGEN_ADDR", "")
        self.bloom = os.environ.get("BLOOM_ADDR", "")
        self.store = os.environ.get("STORE_ADDR", "")

    def ensure_bloom(self):
        with self.lock:
            if self.ready:
                return
            try:
                rpc(self.bloom, "create", {"filter_id": "codes", "m": 8192, "k": 4})
            except GWError as e:
                if e.code != "FILTER_EXISTS":
                    raise
            self.ready = True

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "shorten":
            self.ensure_bloom()
            url = params.get("url")
            if not url:
                raise GWError("INVALID_PARAMS", "url required")
            code = rpc(self.idgen, "next_id", {})["id"]
            rpc(self.bloom, "add", {"filter_id": "codes", "item": code})
            rpc(self.store, "put", {"key": f"links/{code}", "body": url})
            return {"code": code}
        if method == "resolve":
            self.ensure_bloom()
            code = params.get("code")
            if not code:
                raise GWError("INVALID_PARAMS", "code required")
            if not rpc(self.bloom, "contains", {"filter_id": "codes", "item": code}).get("maybe_present"):
                return {"found": False}
            try:
                got = rpc(self.store, "get", {"key": f"links/{code}"})
            except GWError as e:
                if e.code == "NOT_FOUND":
                    return {"found": False}
                raise
            if not got.get("found"):
                return {"found": False}
            return {"found": True, "url": got["body"]}
        if method == "record_click":
            self.ensure_bloom()
            code = params.get("code")
            if not code:
                raise GWError("INVALID_PARAMS", "code required")
            rpc(self.store, "put", {"key": f"clicks/{code}", "body": str(int(time.time() * 1000))})
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
    p.add_argument("--data-dir", required=False)
    args = p.parse_args()
    Server(("127.0.0.1", args.port), Handler).serve_forever()


if __name__ == "__main__":
    main()
