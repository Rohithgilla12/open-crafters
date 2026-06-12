"""Reference solution for the open-crafters "Build your own WAL" challenge.

A key-value store made durable by a write-ahead log:
  - record format: crc32(4, LE) | length(4, LE) | JSON payload  (see PROTOCOL.md)
  - fsync before acknowledging any write
  - recovery stops at the first invalid record and truncates the torn/corrupt
    tail before accepting new appends
  - checkpoint: atomically snapshot full state, then reset the log

Passes all 9 stages.
"""

import argparse
import json
import os
import socketserver
import struct
import threading
import zlib


class EngineError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def encode_record(payload_bytes):
    header_len = struct.pack("<I", len(payload_bytes))
    crc = zlib.crc32(header_len + payload_bytes) & 0xFFFFFFFF
    return struct.pack("<I", crc) + header_len + payload_bytes


class Store:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.wal_path = os.path.join(data_dir, "wal.log")
        self.snapshot_path = os.path.join(data_dir, "snapshot.json")
        self.data = {}
        self._recover()
        self.wal = open(self.wal_path, "ab")

    # ----- recovery -----

    def _recover(self):
        if os.path.exists(self.snapshot_path):
            with open(self.snapshot_path) as f:
                self.data = dict(json.load(f)["data"])

        if not os.path.exists(self.wal_path):
            return
        with open(self.wal_path, "rb") as f:
            raw = f.read()
        offset = 0
        valid_end = 0
        while offset + 8 <= len(raw):
            stored_crc, length = struct.unpack_from("<II", raw, offset)
            if offset + 8 + length > len(raw):
                break  # torn payload
            framed = raw[offset + 4 : offset + 8 + length]
            if zlib.crc32(framed) & 0xFFFFFFFF != stored_crc:
                break  # corrupt record: stop replay here
            record = json.loads(raw[offset + 8 : offset + 8 + length])
            if record["op"] == "set":
                self.data[record["key"]] = record["value"]
            elif record["op"] == "del":
                self.data.pop(record["key"], None)
            offset += 8 + length
            valid_end = offset
        if valid_end < len(raw):
            # Truncate the torn/corrupt tail so the log parses cleanly from
            # byte 0 and new appends don't land after garbage.
            with open(self.wal_path, "r+b") as f:
                f.truncate(valid_end)
                f.flush()
                os.fsync(f.fileno())

    # ----- durable append -----

    def _append(self, payload):
        self.wal.write(encode_record(json.dumps(payload).encode()))
        self.wal.flush()
        os.fsync(self.wal.fileno())

    # ----- RPC methods -----

    def ping(self, params):
        return {"message": "pong"}

    def set(self, params):
        with self.lock:
            self._append({"op": "set", "key": params["key"], "value": params["value"]})
            self.data[params["key"]] = params["value"]
            return {}

    def get(self, params):
        with self.lock:
            if params["key"] in self.data:
                return {"value": self.data[params["key"]], "found": True}
            return {"value": None, "found": False}

    def delete(self, params):
        with self.lock:
            existed = params["key"] in self.data
            self._append({"op": "del", "key": params["key"]})
            self.data.pop(params["key"], None)
            return {"deleted": existed}

    def checkpoint(self, params):
        with self.lock:
            # Snapshot must be durable BEFORE the log is reset: a crash in
            # between just replays the old log onto the new snapshot, which
            # is harmless because set/del are absolute.
            tmp = self.snapshot_path + ".tmp"
            with open(tmp, "w") as f:
                json.dump({"data": self.data}, f)
                f.flush()
                os.fsync(f.fileno())
            os.replace(tmp, self.snapshot_path)

            self.wal.close()
            self.wal = open(self.wal_path, "wb")  # truncate to empty
            self.wal.flush()
            os.fsync(self.wal.fileno())
            self.wal.close()
            self.wal = open(self.wal_path, "ab")
            return {}


METHODS = {
    "ping": "ping",
    "set": "set",
    "get": "get",
    "del": "delete",
    "checkpoint": "checkpoint",
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
                    raise EngineError("UNKNOWN_METHOD",
                                      f"unknown method {request.get('method')!r}")
                result = getattr(store, method)(request.get("params") or {})
                response = {"id": request_id, "result": result}
            except EngineError as e:
                response = {"id": request_id, "error": {"code": e.code, "message": e.message}}
            except Exception as e:
                response = {"id": request_id,
                            "error": {"code": "BAD_REQUEST", "message": str(e)}}
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
    print(f"kv store listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
