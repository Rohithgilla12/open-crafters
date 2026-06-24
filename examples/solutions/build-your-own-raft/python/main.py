"""Reference solution for the open-crafters "Build your own Raft" challenge.

A minimal but correct 3-node Raft cluster:
  - newline-delimited JSON over TCP (see PROTOCOL.md)
  - leader election, log replication, quorum commit
  - crash-safe persistence (atomic JSON snapshot per state change)
  - partition safety (NOT_COMMITTED when quorum unreachable)

Passes all 9 stages. One big lock, election/heartbeat background thread.
"""

import argparse
import json
import os
import random
import socket
import socketserver
import threading
import time
import uuid


class RaftError(Exception):
    def __init__(self, code, message, **extra):
        super().__init__(message)
        self.code = code
        self.message = message
        self.extra = extra


class RaftNode:
    def __init__(self, node_id, peers, data_dir):
        self.node_id = node_id
        self.peers = peers  # id -> host:port
        self.peer_ids = sorted(peers.keys(), key=int)
        self.quorum = len(self.peer_ids) // 2 + 1
        self.lock = threading.Lock()
        self.state_path = os.path.join(data_dir, "state.json")

        self.current_term = 0
        self.voted_for = None
        self.log = []
        self.commit_index = 0
        self.last_applied = 0
        self.kv = {}

        self.role = "follower"
        self.leader_id = "0"
        self.next_index = {}
        self.match_index = {}
        self.votes_received = set()
        self.election_deadline = 0.0
        self.last_quorum_contact = 0.0

        os.makedirs(data_dir, exist_ok=True)
        self._load()
        self.reset_election_timer()

    # ----- persistence -----

    def _load(self):
        if not os.path.exists(self.state_path):
            return
        with open(self.state_path) as f:
            state = json.load(f)
        self.current_term = state.get("term", 0)
        self.voted_for = state.get("voted_for")
        self.log = state.get("log", [])
        self.commit_index = state.get("commit_index", 0)
        self.last_applied = state.get("last_applied", 0)
        self.kv = state.get("kv", {})

    def _persist(self):
        state = {
            "term": self.current_term,
            "voted_for": self.voted_for,
            "log": self.log,
            "commit_index": self.commit_index,
            "last_applied": self.last_applied,
            "kv": self.kv,
        }
        tmp = self.state_path + ".tmp"
        with open(tmp, "w") as f:
            json.dump(state, f)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, self.state_path)

    # ----- helpers -----

    def last_log_index(self):
        return len(self.log)

    def last_log_term(self):
        if not self.log:
            return 0
        return self.log[-1]["term"]

    def reset_election_timer(self):
        self.election_deadline = time.time() + random.uniform(0.3, 0.5)

    def step_down(self, term):
        self.current_term = term
        self.role = "follower"
        self.voted_for = None
        self.leader_id = "0"
        self._persist()
        self.reset_election_timer()

    def become_leader(self):
        self.role = "leader"
        self.leader_id = self.node_id
        self.last_quorum_contact = time.time()
        last_idx = self.last_log_index()
        for pid in self.peer_ids:
            if pid != self.node_id:
                self.next_index[pid] = last_idx + 1
                self.match_index[pid] = 0

    def _maybe_step_down_leader(self):
        if self.role != "leader":
            return
        if time.time() - self.last_quorum_contact > 0.5:
            self.role = "follower"
            self.leader_id = "0"
            self.reset_election_timer()

    def _apply_committed(self):
        while self.last_applied < self.commit_index:
            self.last_applied += 1
            entry = self.log[self.last_applied - 1]
            self.kv[entry["key"]] = entry["value"]
        self._persist()

    def _update_commit_index(self):
        for n in range(self.last_log_index(), self.commit_index, -1):
            count = 1
            for pid in self.peer_ids:
                if pid != self.node_id and self.match_index.get(pid, 0) >= n:
                    count += 1
            if count >= self.quorum and self.log[n - 1]["term"] == self.current_term:
                self.commit_index = n
                self._apply_committed()
                break

    # ----- peer RPC -----

    def _rpc(self, peer_id, method, params, timeout=0.5):
        addr = self.peers.get(peer_id)
        if not addr:
            return None
        try:
            host, port = parse_addr(addr)
            with socket.create_connection((host, port), timeout=timeout) as sock:
                req_id = uuid.uuid4().hex
                payload = json.dumps({
                    "id": req_id,
                    "method": method,
                    "params": params,
                }) + "\n"
                sock.sendall(payload.encode())
                buf = b""
                while b"\n" not in buf:
                    chunk = sock.recv(65536)
                    if not chunk:
                        break
                    buf += chunk
                if not buf:
                    return None
                resp = json.loads(buf.decode().split("\n")[0])
                if resp.get("error"):
                    return None
                return resp.get("result")
        except OSError:
            return None

    # ----- election / replication -----

    def _start_election(self):
        with self.lock:
            self.role = "candidate"
            self.current_term += 1
            self.voted_for = self.node_id
            self.leader_id = "0"
            self.votes_received = {self.node_id}
            self._persist()
            term = self.current_term
            last_idx = self.last_log_index()
            last_term = self.last_log_term()
            self.reset_election_timer()

        for pid in self.peer_ids:
            if pid != self.node_id:
                threading.Thread(
                    target=self._request_vote,
                    args=(pid, term, last_idx, last_term),
                    daemon=True,
                ).start()

    def _request_vote(self, peer_id, term, last_idx, last_term):
        result = self._rpc(peer_id, "request_vote", {
            "term": term,
            "candidate_id": self.node_id,
            "last_log_index": last_idx,
            "last_log_term": last_term,
        })
        if result is None:
            return
        with self.lock:
            if result["term"] > self.current_term:
                self.step_down(result["term"])
                return
            if self.role != "candidate" or self.current_term != term:
                return
            if result["vote_granted"]:
                self.votes_received.add(peer_id)
                if len(self.votes_received) >= self.quorum:
                    self.become_leader()

    def _replicate_to(self, peer_id):
        with self.lock:
            if self.role != "leader":
                return
            next_idx = self.next_index.get(peer_id, 1)
            prev_log_index = next_idx - 1
            prev_log_term = 0
            if prev_log_index > 0:
                prev_log_term = self.log[prev_log_index - 1]["term"]
            entries = self.log[next_idx - 1:] if next_idx <= self.last_log_index() else []
            term = self.current_term
            leader_commit = self.commit_index

        result = self._rpc(peer_id, "append_entries", {
            "term": term,
            "leader_id": self.node_id,
            "prev_log_index": prev_log_index,
            "prev_log_term": prev_log_term,
            "entries": entries,
            "leader_commit": leader_commit,
        })
        if result is None:
            return

        with self.lock:
            if result["term"] > self.current_term:
                self.step_down(result["term"])
                return
            if self.role != "leader" or self.current_term != term:
                return
            if result["success"]:
                self.last_quorum_contact = time.time()
                if entries:
                    self.match_index[peer_id] = next_idx + len(entries) - 1
                    self.next_index[peer_id] = self.match_index[peer_id] + 1
                self._update_commit_index()
            else:
                if self.next_index.get(peer_id, 1) > 1:
                    self.next_index[peer_id] = self.next_index[peer_id] - 1

    def _leader_heartbeat(self):
        for pid in self.peer_ids:
            if pid != self.node_id:
                threading.Thread(target=self._replicate_to, args=(pid,), daemon=True).start()

    def run_raft_loop(self):
        last_heartbeat = 0.0
        while True:
            time.sleep(0.05)
            now = time.time()
            need_election = False
            with self.lock:
                if self.role == "leader":
                    self._maybe_step_down_leader()
                    if self.role == "leader" and now - last_heartbeat >= 0.1:
                        last_heartbeat = now
                        self._leader_heartbeat()
                elif now >= self.election_deadline:
                    need_election = True
            if need_election:
                self._start_election()

    # ----- client / Raft RPC methods -----

    def ping(self, params):
        return {"message": "pong", "node_id": self.node_id}

    def get_status(self, params):
        with self.lock:
            return {
                "node_id": self.node_id,
                "role": self.role,
                "term": self.current_term,
                "leader_id": self.leader_id,
                "commit_index": self.commit_index,
                "last_applied": self.last_applied,
            }

    def set(self, params):
        with self.lock:
            if self.role != "leader":
                raise RaftError(
                    "NOT_LEADER",
                    "not the leader",
                    leader_id=self.leader_id or "0",
                )
            index = self.last_log_index() + 1
            entry = {
                "index": index,
                "term": self.current_term,
                "key": params["key"],
                "value": params["value"],
            }
            self.log.append(entry)
            self._persist()
            target_index = index

        self._leader_heartbeat()

        deadline = time.time() + 1.5
        while time.time() < deadline:
            with self.lock:
                if self.commit_index >= target_index:
                    return {"index": target_index}
                if self.role != "leader":
                    raise RaftError(
                        "NOT_LEADER",
                        "not the leader",
                        leader_id=self.leader_id or "0",
                    )
            time.sleep(0.01)
            self._leader_heartbeat()

        raise RaftError("NOT_COMMITTED", "could not replicate to a quorum")

    def get(self, params):
        with self.lock:
            self._apply_committed()
            key = params["key"]
            if key in self.kv:
                return {"found": True, "value": self.kv[key]}
            return {"found": False}

    def request_vote(self, params):
        with self.lock:
            term = params["term"]
            candidate_id = params["candidate_id"]
            last_log_index = params["last_log_index"]
            last_log_term = params["last_log_term"]

            if term < self.current_term:
                return {"term": self.current_term, "vote_granted": False}

            if term > self.current_term:
                self.step_down(term)

            up_to_date = (
                last_log_term > self.last_log_term()
                or (
                    last_log_term == self.last_log_term()
                    and last_log_index >= self.last_log_index()
                )
            )

            vote_granted = False
            if up_to_date and (
                self.voted_for is None or self.voted_for == candidate_id
            ):
                self.voted_for = candidate_id
                vote_granted = True
                self._persist()

            self.reset_election_timer()
            return {"term": self.current_term, "vote_granted": vote_granted}

    def append_entries(self, params):
        with self.lock:
            term = params["term"]
            leader_id = params["leader_id"]

            if term < self.current_term:
                return {"term": self.current_term, "success": False}

            if term > self.current_term:
                self.step_down(term)

            self.role = "follower"
            self.leader_id = leader_id
            self.reset_election_timer()

            prev_log_index = params["prev_log_index"]
            prev_log_term = params["prev_log_term"]

            if prev_log_index > 0:
                if (
                    prev_log_index > self.last_log_index()
                    or self.log[prev_log_index - 1]["term"] != prev_log_term
                ):
                    return {"term": self.current_term, "success": False}

            entries = params["entries"]
            if entries:
                self.log = self.log[:prev_log_index]
                for entry in entries:
                    self.log.append(entry)
                self._persist()

            leader_commit = params["leader_commit"]
            if leader_commit > self.commit_index:
                self.commit_index = min(leader_commit, self.last_log_index())
                self._apply_committed()

            return {"term": self.current_term, "success": True}


METHODS = frozenset({
    "ping", "get_status", "set", "get",
    "request_vote", "append_entries",
})


def parse_addr(addr_str):
    host, port_str = addr_str.rsplit(":", 1)
    return host, int(port_str)


def parse_peers(peers_str):
    peers = {}
    for part in peers_str.split(","):
        part = part.strip()
        if not part:
            continue
        node_id, addr = part.split("=", 1)
        peers[node_id] = addr
    return peers


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        node = self.server.node
        for line in self.rfile:
            line = line.strip()
            if not line:
                continue
            request_id = None
            try:
                request = json.loads(line)
                request_id = request.get("id")
                method = request.get("method")
                if method not in METHODS:
                    raise RaftError("UNKNOWN_METHOD", f"unknown method {method!r}")
                result = getattr(node, method)(request.get("params") or {})
                response = {"id": request_id, "result": result}
            except RaftError as e:
                err = {"code": e.code, "message": e.message}
                err.update(e.extra)
                response = {"id": request_id, "error": err}
            except Exception as e:
                response = {
                    "id": request_id,
                    "error": {"code": "BAD_REQUEST", "message": str(e)},
                }
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

    peers = parse_peers(args.peers)
    node = RaftNode(args.node_id, peers, args.data_dir)
    threading.Thread(target=node.run_raft_loop, daemon=True).start()

    server = Server(("127.0.0.1", args.port), Handler)
    server.node = node
    print(f"raft node {args.node_id} listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
