"""Starter template for "Build your own distributed lock" (Python). Passes stage 1 only."""

import argparse
import json
import socketserver


class DistLockError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def handle_request(method, params):
    if method == "ping":
        return {"message": "pong"}
    # TODO (stage 2): acquire + status
    # TODO (stage 3): release with token check
    # TODO (stage 4): LOCK_HELD on contended acquire
    # TODO (stage 5): try_acquire (acquired:false, no error on contention)
    # TODO (stage 6): lazy expiry via expires_at_ms
    # TODO (stage 7): renew + NOT_HOLDER
    # TODO (stage 8): persist active locks to --data-dir
    raise DistLockError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except DistLockError as e:
                response = {"id": request.get("id"), "error": {"code": e.code, "message": e.message}}
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
