"""Reference solution for the open-crafters "Build your own MVCC" challenge.

A transactional key-value store with multi-version concurrency control:
  - each begin() captures a snapshot (the latest committed sequence number)
  - reads see the newest version <= the snapshot, overlaid by the txn's own
    buffered writes (read-your-writes)
  - commit assigns a monotonic sequence number and is durable before it is
    acknowledged; a write-write conflict (a key we wrote was committed by
    someone else after our snapshot) aborts with CONFLICT
  - recovery replays the commit log to rebuild every key's version history

Isolation level is snapshot isolation: lost updates are prevented (write-write
conflict), but write skew is permitted. Passes all 9 stages.
"""

import argparse
import json
import os
import socketserver
import threading
import uuid


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Store:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.log_path = os.path.join(data_dir, "commits.log")
        # key -> list of (seq, value) ordered by seq; value None means tombstone.
        self.versions = {}
        self.commit_seq = 0
        self.txns = {}  # txn id -> {"snapshot": int, "writes": {key: value|None}}
        self._recover()
        self.log = open(self.log_path, "ab")

    # ----- recovery -----

    def _apply(self, seq, writes):
        for key, value in writes.items():
            self.versions.setdefault(key, []).append((seq, value))
        self.commit_seq = max(self.commit_seq, seq)

    def _recover(self):
        if not os.path.exists(self.log_path):
            return
        with open(self.log_path) as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                rec = json.loads(line)
                self._apply(rec["seq"], rec["writes"])

    # ----- version reads -----

    def _read_committed(self, key, snapshot):
        """Newest committed (value, found) for key as of snapshot."""
        best = None
        for seq, value in self.versions.get(key, ()):
            if seq <= snapshot:
                best = value  # versions are appended in increasing seq order
        if best is None:
            return None, False  # absent or tombstone
        return best, True

    def _txn(self, params):
        t = self.txns.get(params.get("txn"))
        if t is None:
            raise RPCError("UNKNOWN_TXN", f"no open transaction {params.get('txn')!r}")
        return t

    # ----- RPC methods -----

    def ping(self, params):
        return {"message": "pong"}

    def begin(self, params):
        with self.lock:
            tid = uuid.uuid4().hex
            self.txns[tid] = {"snapshot": self.commit_seq, "writes": {}}
            return {"txn": tid}

    def get(self, params):
        with self.lock:
            t = self._txn(params)
            key = params["key"]
            if key in t["writes"]:
                value = t["writes"][key]
                if value is None:
                    return {"value": None, "found": False}
                return {"value": value, "found": True}
            value, found = self._read_committed(key, t["snapshot"])
            return {"value": value if found else None, "found": found}

    def set(self, params):
        with self.lock:
            t = self._txn(params)
            t["writes"][params["key"]] = params["value"]
            return {}

    def delete(self, params):
        with self.lock:
            t = self._txn(params)
            t["writes"][params["key"]] = None  # buffered tombstone
            return {}

    def commit(self, params):
        with self.lock:
            t = self._txn(params)
            # Snapshot-isolation write-write conflict: a key we wrote was
            # committed by someone else after our snapshot → abort.
            for key in t["writes"]:
                history = self.versions.get(key)
                if history and history[-1][0] > t["snapshot"]:
                    del self.txns[params["txn"]]
                    raise RPCError("CONFLICT",
                                   f"key {key!r} was modified by a concurrent transaction")
            if t["writes"]:
                self.commit_seq += 1
                seq = self.commit_seq
                self._persist(seq, t["writes"])
                self._apply(seq, t["writes"])
            del self.txns[params["txn"]]
            return {"committed": True}

    def rollback(self, params):
        with self.lock:
            self._txn(params)
            del self.txns[params["txn"]]
            return {}

    def _persist(self, seq, writes):
        self.log.write((json.dumps({"seq": seq, "writes": writes}) + "\n").encode())
        self.log.flush()
        os.fsync(self.log.fileno())


METHODS = {
    "ping": "ping",
    "begin": "begin",
    "get": "get",
    "set": "set",
    "delete": "delete",
    "commit": "commit",
    "rollback": "rollback",
}


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        store = self.server.store
        for line in self.rfile:
            line = line.strip()
            if not line:
                continue
            request_id = None
            try:
                request = json.loads(line)
                request_id = request.get("id")
                method = METHODS.get(request.get("method"))
                if method is None:
                    raise RPCError("UNKNOWN_METHOD", f"unknown method {request.get('method')!r}")
                result = getattr(store, method)(request.get("params") or {})
                response = {"id": request_id, "result": result}
            except RPCError as e:
                response = {"id": request_id, "error": {"code": e.code, "message": e.message}}
            except Exception as e:
                response = {"id": request_id, "error": {"code": "BAD_REQUEST", "message": str(e)}}
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
    server.store = Store(args.data_dir)
    print(f"mvcc store listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
