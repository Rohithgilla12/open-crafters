"""Starter for notification platform gateway (Python). Passes bind only."""

import argparse
import json
import os
import socketserver


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def handle(method, params):
    if method == "ping":
        return {"message": "pong"}
    _ = os.environ.get("QUEUE_ADDR")
    _ = os.environ.get("SCHEDULER_ADDR")
    _ = os.environ.get("RATE_LIMITER_ADDR")
    raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            try:
                result = handle(req.get("method"), req.get("params") or {})
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
