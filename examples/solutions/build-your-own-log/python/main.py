"""Reference solution for the open-crafters "Build your own log" challenge.

An append-only, replayable log (Kafka-style):
  - append assigns a monotonic, absolute offset per topic
  - read is non-destructive and replayable from any offset
  - consumer groups track their own committed offset; reads don't consume
  - retention drops old records WITHOUT renumbering offsets (they stay absolute)
  - appends, truncations, and offset commits are durable across a crash

Passes all 9 stages.
"""

import argparse
import json
import os
import socketserver
import threading


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Topic:
    def __init__(self):
        self.base = 0          # offset of values[0] (rises with retention)
        self.values = []       # retained record values, in offset order

    @property
    def end(self):             # next offset to be assigned
        return self.base + len(self.values)


class Store:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.log_path = os.path.join(data_dir, "log.jsonl")
        self.topics = {}                      # name -> Topic
        self.offsets = {}                     # (group, topic) -> committed offset
        self._recover()
        self.log = open(self.log_path, "ab")

    # ----- recovery -----

    def _apply(self, rec):
        t = rec["t"]
        if t == "append":
            self.topics.setdefault(rec["topic"], Topic()).values.append(rec["value"])
        elif t == "truncate":
            topic = self.topics.get(rec["topic"])
            if topic and rec["before"] > topic.base:
                drop = min(rec["before"], topic.end) - topic.base
                topic.values = topic.values[drop:]
                topic.base += drop
        elif t == "commit":
            self.offsets[(rec["group"], rec["topic"])] = rec["offset"]

    def _recover(self):
        if not os.path.exists(self.log_path):
            return
        with open(self.log_path) as f:
            for line in f:
                line = line.strip()
                if line:
                    self._apply(json.loads(line))

    def _persist(self, rec):
        self.log.write((json.dumps(rec) + "\n").encode())
        self.log.flush()
        os.fsync(self.log.fileno())

    # ----- RPC methods -----

    def ping(self, params):
        return {"message": "pong"}

    def append(self, params):
        with self.lock:
            topic = self.topics.setdefault(params["topic"], Topic())
            offset = topic.end
            self._persist({"t": "append", "topic": params["topic"], "value": params["value"]})
            topic.values.append(params["value"])
            return {"offset": offset}

    def read(self, params):
        with self.lock:
            topic = self.topics.get(params["topic"])
            if topic is None:
                topic = Topic()  # unknown topic reads as empty from offset 0
            offset = params["offset"]
            limit = params.get("max", 100)
            if offset < topic.base:
                raise RPCError("OUT_OF_RANGE",
                               f"offset {offset} is below the earliest retained offset {topic.base}")
            records = []
            i = offset
            while i < topic.end and len(records) < limit:
                records.append({"offset": i, "value": topic.values[i - topic.base]})
                i += 1
            return {"records": records, "next_offset": i}

    def commit_offset(self, params):
        with self.lock:
            self._persist({"t": "commit", "group": params["group"],
                           "topic": params["topic"], "offset": params["offset"]})
            self.offsets[(params["group"], params["topic"])] = params["offset"]
            return {}

    def committed_offset(self, params):
        with self.lock:
            return {"offset": self.offsets.get((params["group"], params["topic"]), 0)}

    def truncate(self, params):
        with self.lock:
            topic = self.topics.setdefault(params["topic"], Topic())
            before = params["before"]
            self._persist({"t": "truncate", "topic": params["topic"], "before": before})
            if before > topic.base:
                drop = min(before, topic.end) - topic.base
                topic.values = topic.values[drop:]
                topic.base += drop
            return {}

    def stats(self, params):
        with self.lock:
            topic = self.topics.get(params["topic"])
            if topic is None:
                return {"start_offset": 0, "end_offset": 0}
            return {"start_offset": topic.base, "end_offset": topic.end}


METHODS = {
    "ping": "ping",
    "append": "append",
    "read": "read",
    "commit_offset": "commit_offset",
    "committed_offset": "committed_offset",
    "truncate": "truncate",
    "stats": "stats",
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
    print(f"log store listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
