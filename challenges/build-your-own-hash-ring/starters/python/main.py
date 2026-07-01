"""Starter template for "Build your own hash ring" (Python). Passes stage 1 only."""

import argparse
import json
import socketserver


class HashRingError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def handle_request(method, params):
    if method == "ping":
        return {"message": "pong"}
    # TODO (stage 2): create_ring + RING_EXISTS / INVALID_PARAMS
    # TODO (stage 3): add_node, lookup + RING_NOT_FOUND / NODE_EXISTS / NO_NODES
    # TODO (stage 4): deterministic FNV-1a vnode walk per PROTOCOL.md
    # TODO (stage 5): even key spread across 3 nodes
    # TODO (stage 6): add 4th node — fewer than 45% of keys move
    # TODO (stage 7): remove_node — keys remap, never return removed node
    # TODO (stage 8): replicas flatten load (virtual nodes)
    # TODO (stage 9): concurrent add/remove/lookup across 2 rings
    raise HashRingError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except HashRingError as e:
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
