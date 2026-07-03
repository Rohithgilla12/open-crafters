"""Starter for Build your own ID generator (Python). Passes stage 1 only."""

import argparse
import json
import socketserver


class IDError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def dispatch(method, _params):
    if method == "ping":
        return {"message": "pong"}
    raise IDError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            line = line.strip()
            if not line:
                continue
            req = json.loads(line)
            try:
                result = dispatch(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except IDError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write((json.dumps(resp) + "\n").encode())
            self.wfile.flush()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=True)
    args = parser.parse_args()
    with socketserver.ThreadingTCPServer(("127.0.0.1", args.port), Handler) as srv:
        print(f"listening on 127.0.0.1:{args.port}", flush=True)
        srv.serve_forever()


if __name__ == "__main__":
    main()
