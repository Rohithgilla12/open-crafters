"""Starter template for "Build your own message queue" (Python).

This boots a TCP server speaking newline-delimited JSON and answers `ping` —
enough to pass the first stage. Extend `handle_request` stage by stage.
See PROTOCOL.md for the wire protocol and the delivery model (the real spec).
"""

import argparse
import json
import socketserver


def handle_request(method, params):
    """Returns a result dict, or raises RPCError."""
    if method == "ping":
        return {"message": "pong"}

    # TODO (stage 2): send / receive / ack — an in-memory FIFO queue
    # TODO (stage 3): persist to --data-dir; un-acked messages survive SIGKILL
    # TODO (stage 4): visibility timeouts — redeliver if not acked in time
    # TODO (stage 5): nack — make a message visible again immediately
    # TODO (stage 6): receipt fencing — a receipt is valid for one delivery
    # TODO (stage 7): configure + dead-letter after max_receives failures
    # TODO (stage 8): many independent queues + stats
    raise RPCError("UNKNOWN_METHOD", f"unknown method {method!r}")


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except RPCError as e:
                response = {"id": request.get("id"),
                            "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(response).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=True)
    args = parser.parse_args()

    server = Server(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
