"""Starter template for "Build your own workflow SDK" (Python).

This boots a TCP server speaking newline-delimited JSON and answers `ping` —
enough to pass the first stage. Extend `handle_request` stage by stage.
See PROTOCOL.md for the full wire protocol.
"""

import argparse
import json
import socketserver


class EngineError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def handle_request(method, params):
    """Returns a result dict, or raises EngineError."""
    if method == "ping":
        return {"message": "pong"}

    # TODO (stage 2): replay — greet workflow → COMPLETE_WORKFLOW
    # TODO (stage 3): fetch workflow → SCHEDULE_ACTIVITY
    # TODO (stage 4): fetch after ACTIVITY_TASK_COMPLETED → COMPLETE_WORKFLOW
    # TODO (stage 5): waiting states → empty commands
    # TODO (stage 6): timer_wait workflow
    # TODO (stage 7): signal_wait workflow
    # TODO (stage 8): determinism — no randomness or wall clock in replay
    # TODO (stage 9): pipeline workflow (gauntlet)
    raise EngineError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except EngineError as e:
                response = {
                    "id": request.get("id"),
                    "error": {"code": e.code, "message": e.message},
                }
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
