"""Reference solution for Build your own ID generator (Python). Passes all 9 stages."""

import argparse
import json
import os
import socketserver
import threading
import time

SNOWFLAKE_EPOCH_MS = 1577836800000
MAX_WORKER_ID = 1023
MAX_SEQUENCE = 4095
MAX_BATCH = 1024


class IDError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def now_ms():
    return int(time.time() * 1000)


def compose_id(ts, worker, seq):
    rel = max(0, ts - SNOWFLAKE_EPOCH_MS)
    return str((rel << 22) | (worker << 12) | seq)


def decode_id(val):
    sequence = val & MAX_SEQUENCE
    worker = (val >> 12) & MAX_WORKER_ID
    rel = val >> 22
    return rel + SNOWFLAKE_EPOCH_MS, worker, sequence


class Engine:
    def __init__(self, data_dir):
        self.state_path = os.path.join(data_dir, "state.json")
        self.lock = threading.Lock()
        self.worker_id = 0
        self.last_ts = 0
        self.last_seq = 0
        self._load()

    def _load(self):
        try:
            with open(self.state_path, encoding="utf-8") as f:
                data = json.load(f)
            self.last_ts = data.get("last_timestamp_ms", 0)
            self.last_seq = data.get("last_sequence", 0)
            self.worker_id = data.get("worker_id", 0)
        except OSError:
            pass

    def _persist(self):
        tmp = self.state_path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(
                {
                    "last_timestamp_ms": self.last_ts,
                    "last_sequence": self.last_seq,
                    "worker_id": self.worker_id,
                },
                f,
            )
        os.replace(tmp, self.state_path)

    def _wait_next_ms(self, last):
        while True:
            n = now_ms()
            if n > last:
                return n
            time.sleep(0.001)

    def _alloc_in_bucket(self, bucket):
        if bucket[0] < self.last_ts:
            raise IDError("CLOCK_BACKWARDS", "clock moved backwards")
        if self.last_ts == bucket[0]:
            if self.last_seq >= MAX_SEQUENCE:
                bucket[0] = self._wait_next_ms(bucket[0])
                if bucket[0] < self.last_ts:
                    raise IDError("CLOCK_BACKWARDS", "clock moved backwards")
                self.last_ts = bucket[0]
                self.last_seq = 0
            else:
                self.last_seq += 1
        else:
            self.last_ts = bucket[0]
            self.last_seq = 0
        return compose_id(self.last_ts, self.worker_id, self.last_seq)

    def configure(self, worker_id):
        if worker_id < 0 or worker_id > MAX_WORKER_ID:
            raise IDError("INVALID_PARAMS", "worker_id must be 0..1023")
        with self.lock:
            self.worker_id = worker_id
            self._persist()

    def next_id(self):
        with self.lock:
            bucket = [now_ms()]
            id_ = self._alloc_in_bucket(bucket)
            self._persist()
            return id_

    def batch(self, count):
        if count < 1:
            raise IDError("INVALID_PARAMS", "count must be >= 1")
        if count > MAX_BATCH:
            raise IDError("BATCH_TOO_LARGE", f"count must be <= {MAX_BATCH}")
        with self.lock:
            bucket = [now_ms()]
            ids = [self._alloc_in_bucket(bucket) for _ in range(count)]
            self._persist()
            return ids

    def parse(self, id_str):
        try:
            val = int(id_str)
        except ValueError:
            raise IDError("INVALID_PARAMS", "id must be a positive decimal integer") from None
        if val <= 0:
            raise IDError("INVALID_PARAMS", "id must be a positive decimal integer")
        ts, worker, seq = decode_id(val)
        return {"timestamp_ms": ts, "worker_id": worker, "sequence": seq}


class Handler(socketserver.StreamRequestHandler):
    engine = None

    def handle(self):
        for line in self.rfile:
            line = line.strip()
            if not line:
                continue
            req = json.loads(line)
            try:
                result = dispatch(self.engine, req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except IDError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write((json.dumps(resp) + "\n").encode())
            self.wfile.flush()


def dispatch(engine, method, params):
    if method == "ping":
        return {"message": "pong"}
    if method == "configure":
        if "worker_id" not in params:
            raise IDError("INVALID_PARAMS", "configure requires worker_id")
        engine.configure(int(params["worker_id"]))
        return {}
    if method == "next_id":
        return {"id": engine.next_id()}
    if method == "batch":
        if "count" not in params:
            raise IDError("INVALID_PARAMS", "batch requires count")
        return {"ids": engine.batch(int(params["count"]))}
    if method == "parse":
        if not params.get("id"):
            raise IDError("INVALID_PARAMS", "parse requires id")
        return engine.parse(params["id"])
    raise IDError("UNKNOWN_METHOD", f"unknown method {method!r}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=True)
    args = parser.parse_args()
    Handler.engine = Engine(args.data_dir)
    with socketserver.ThreadingTCPServer(("127.0.0.1", args.port), Handler) as srv:
        print(f"listening on 127.0.0.1:{args.port}", flush=True)
        srv.serve_forever()


if __name__ == "__main__":
    main()
