"""Starter template for "Build your own bloom filter" (Python). Passes stage 1 only."""

import argparse
import json
import socketserver


class BloomFilterError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def handle_request(method, params):
    if method == "ping":
        return {"message": "pong"}
    # TODO (stage 2): create filter (m bits, k hashes) + FILTER_EXISTS / INVALID_PARAMS
    # TODO (stage 3): add item via FNV-1a double hash + FILTER_NOT_FOUND
    # TODO (stage 4): contains → maybe_present (all k bits set)
    # TODO (stage 5): sparse filter — never-added items usually false
    # TODO (stage 6): no false negatives under bulk insert
    # TODO (stage 7): independent filters per filter_id
    # TODO (stage 8): all k positions — (h1 + i*h2) % m, not just h1 % m
    # TODO (stage 9): concurrent add/contains + optional delete_filter
    raise BloomFilterError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except BloomFilterError as e:
                response = {"id": request.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(response).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=False)  # ignored; harness may pass it
    args = parser.parse_args()
    _ = args.data_dir
    server = Server(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
