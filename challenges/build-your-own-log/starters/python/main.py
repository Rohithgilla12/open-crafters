"""Starter template for "Build your own log" (Python).

Boots a TCP server speaking newline-delimited JSON and answers `ping` — enough
to pass the first stage. Extend `handle_request` stage by stage.
See PROTOCOL.md for the wire protocol and the log model (the real spec).
"""

import argparse
import json
import socketserver


def handle_request(method, params):
    """Returns a result dict, or raises RPCError."""
    if method == "ping":
        return {"message": "pong"}

    # TODO (stage 2): append / read — monotonic 0-based offsets per topic
    # TODO (stage 3): persist to --data-dir (records + offsets survive a crash)
    # TODO (stage 4): multiple independent topics
    # TODO (stage 5): commit_offset / committed_offset (consumer groups)
    # TODO (stage 6): read `max` batching; reads are replayable, non-destructive
    # TODO (stage 7): truncate — retention that keeps offsets ABSOLUTE
    # TODO (stage 8): persist committed offsets and retention state
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
