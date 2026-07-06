"""Starter URL shortener gateway (Python). Bind only."""

import argparse
import json
import socketserver


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            try:
                if req.get("method") == "ping":
                    resp = {"id": req.get("id"), "result": {"message": "pong"}}
                else:
                    raise GWError("UNKNOWN_METHOD", "not implemented")
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
