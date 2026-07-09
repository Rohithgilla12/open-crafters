"""Starter for chat service gateway (Python). Passes bind only."""

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
    _ = os.environ.get("IDGEN_ADDR")
    _ = os.environ.get("LOG_ADDR")
    _ = os.environ.get("QUEUE_ADDR")
    # TODO: send_message, read_messages, poll_delivery, ack_delivery
    raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            line = line.decode().strip()
            if not line:
                continue
            req = json.loads(line)
            try:
                result = handle(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except GWError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
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
