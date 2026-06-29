"""Starter template for "Build your own object store" (Python). Passes stage 1 only."""

import argparse
import json
import socketserver


class ObjectStoreError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def handle_request(method, params):
    if method == "ping":
        return {"message": "pong"}
    # TODO (stage 2): put / get with SHA-256 etags
    # TODO (stage 3): head — metadata only
    # TODO (stage 4): overwrite on put
    # TODO (stage 5): delete
    # TODO (stage 6): list by prefix
    # TODO (stage 7): multipart upload
    # TODO (stage 8): persist to --data-dir
    raise ObjectStoreError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except ObjectStoreError as e:
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
