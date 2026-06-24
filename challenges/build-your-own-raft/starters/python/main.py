"""Starter template for "Build your own Raft" (Python).

Boots a TCP server speaking newline-delimited JSON and answers `ping` with the
node id — enough to pass the first stage. Extend stage by stage.
See PROTOCOL.md for the full wire protocol.
"""

import argparse
import json
import socketserver


class RaftError(Exception):
    def __init__(self, code, message, **extra):
        super().__init__(message)
        self.code = code
        self.message = message
        self.extra = extra


def parse_peers(peers_str):
    peers = {}
    for part in peers_str.split(","):
        part = part.strip()
        if not part:
            continue
        node_id, addr = part.split("=", 1)
        peers[node_id] = addr
    return peers


def handle_request(method, params, node_id):
    """Returns a result dict, or raises RaftError."""
    if method == "ping":
        return {"message": "pong", "node_id": node_id}

    # TODO (stage 2): leader election — get_status, request_vote, append_entries heartbeats
    # TODO (stage 3): set on leader, replicate log entries to a quorum
    # TODO (stage 4): get — serve reads from committed/applied state
    # TODO (stage 5): tolerate a follower crash (majority still commits)
    # TODO (stage 6): re-elect after leader crash, preserve committed log
    # TODO (stage 7): persist term, voted_for, log, commit_index, KV to --data-dir
    # TODO (stage 8): partition safety — NOT_COMMITTED when quorum unreachable
    # TODO (stage 9): gauntlet — writes, crash, restart
    raise RaftError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        node_id = self.server.node_id
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(
                    request.get("method"),
                    request.get("params") or {},
                    node_id,
                )
                response = {"id": request.get("id"), "result": result}
            except RaftError as e:
                err = {"code": e.code, "message": e.message}
                err.update(e.extra)
                response = {"id": request.get("id"), "error": err}
            self.wfile.write(json.dumps(response).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--node-id", required=True)
    parser.add_argument("--peers", required=True)
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=True)
    args = parser.parse_args()
    _ = parse_peers(args.peers)
    _ = args.data_dir

    server = Server(("127.0.0.1", args.port), Handler)
    server.node_id = args.node_id
    print(f"raft node {args.node_id} listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
