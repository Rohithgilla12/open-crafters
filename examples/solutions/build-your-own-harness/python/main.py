"""Reference solution for build-your-own-harness (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import subprocess
import sys
import tempfile
import threading
import time


class HarnessError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def wait_tcp(addr, timeout=10.0):
    host, port = addr.split(":")
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            c = socket.create_connection((host, int(port)), timeout=0.25)
            c.close()
            return
        except OSError:
            time.sleep(0.1)
    raise HarnessError("SPAWN_FAILED", f"timeout waiting for {addr}")


def proxy_call(addr, method, params):
    host, port = addr.split(":")
    conn = socket.create_connection((host, int(port)), timeout=10)
    try:
        conn.sendall((json.dumps({"id": "1", "method": method, "params": params or {}}) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = conn.recv(4096)
            if not chunk:
                break
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise HarnessError(e["code"], e["message"])
        return resp.get("result", {})
    finally:
        conn.close()


def subset_match(got, expect):
    for k, wv in expect.items():
        if got.get(k) != wv:
            return False
    return True


class Engine:
    def __init__(self):
        self.lock = threading.Lock()
        self.children = []

    def spawn(self, program):
        if not program:
            raise HarnessError("INVALID_PARAMS", "spawn requires program")
        s = socket.socket()
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]
        s.close()
        data_dir = tempfile.mkdtemp(prefix="harness-child-")
        proc = subprocess.Popen(
            [program, "--port", str(port), "--data-dir", data_dir],
            cwd=os.path.dirname(program),
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
        with self.lock:
            self.children.append((proc, data_dir))
        addr = f"127.0.0.1:{port}"
        try:
            wait_tcp(addr)
        except HarnessError:
            proc.kill()
            proc.wait()
            os.rmdir(data_dir)
            raise
        return addr

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "spawn":
            program = params.get("program")
            return {"addr": self.spawn(program)}
        if method == "call":
            addr = params.get("addr")
            m = params.get("method")
            if not addr or not m:
                raise HarnessError("INVALID_PARAMS", "call requires addr and method")
            return proxy_call(addr, m, params.get("params"))
        if method == "run_case":
            program = params.get("program")
            m = params.get("method")
            expect = params.get("expect")
            if not program or not m or expect is None:
                raise HarnessError("INVALID_PARAMS", "run_case requires program, method, expect")
            addr = self.spawn(program)
            got = proxy_call(addr, m, params.get("params"))
            if not subset_match(got, expect):
                raise HarnessError("CASE_FAILED", f"got {got} expect subset {expect}")
            return {}
        raise HarnessError("UNKNOWN_METHOD", f"unknown method {method!r}")


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
            except HarnessError as e:
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
