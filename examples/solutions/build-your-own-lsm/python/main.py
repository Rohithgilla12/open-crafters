"""Reference solution for the open-crafters "Build your own LSM-tree" challenge.

An LSM-tree key-value store:
  - memtable for in-memory writes
  - SST1 on-disk format (see PROTOCOL.md)
  - flush, scan, compact, tombstones
  - fsync before acknowledging flush/compact

Passes all 9 stages.
"""

import argparse
import json
import os
import socketserver
import struct
import threading


SST_MAGIC = b"SST1"


class EngineError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def encode_sst(entries):
    """entries: sorted list of (key, value, deleted)."""
    body = bytearray()
    body += struct.pack("<I", len(entries))
    for key, value, deleted in entries:
        key_b = key.encode()
        val_b = b"" if deleted else value.encode()
        body += struct.pack("<I", len(key_b))
        body += key_b
        body += struct.pack("<I", len(val_b))
        body += val_b
    return SST_MAGIC + bytes(body)


def parse_sst(data):
    if len(data) < 8 or data[:4] != SST_MAGIC:
        raise ValueError("invalid SST file")
    count = struct.unpack_from("<I", data, 4)[0]
    offset = 8
    entries = []
    for _ in range(count):
        (key_len,) = struct.unpack_from("<I", data, offset)
        offset += 4
        key = data[offset : offset + key_len].decode()
        offset += key_len
        (val_len,) = struct.unpack_from("<I", data, offset)
        offset += 4
        val = data[offset : offset + val_len].decode()
        offset += val_len
        entries.append((key, val, val_len == 0))
    return entries


class Store:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.sst_dir = os.path.join(data_dir, "sst")
        os.makedirs(self.sst_dir, exist_ok=True)
        self.mem = {}  # key -> (value, deleted)
        self.sst_files = []
        self.next_seq = 1
        self._load_index()

    def _load_index(self):
        names = sorted(f for f in os.listdir(self.sst_dir) if f.endswith(".sst"))
        self.sst_files = [os.path.join(self.sst_dir, n) for n in names]
        if names:
            self.next_seq = int(names[-1][:6]) + 1

    def _write_sst(self, entries):
        path = os.path.join(self.sst_dir, f"{self.next_seq:06d}.sst")
        data = encode_sst(entries)
        with open(path, "wb") as f:
            f.write(data)
            f.flush()
            os.fsync(f.fileno())
        self.sst_files.append(path)
        self.next_seq += 1
        return path

    def _read_sst(self, path):
        with open(path, "rb") as f:
            return parse_sst(f.read())

    def _lookup(self, key):
        if key in self.mem:
            val, deleted = self.mem[key]
            return (None, False) if deleted else (val, True)
        for path in reversed(self.sst_files):
            for k, val, deleted in self._read_sst(path):
                if k == key:
                    return (None, False) if deleted else (val, True)
        return (None, False)

    def _merged_live(self):
        resolved = {}
        for path in self.sst_files:
            for key, val, deleted in self._read_sst(path):
                if deleted:
                    resolved[key] = None
                else:
                    resolved[key] = val
        for key, (val, deleted) in self.mem.items():
            if deleted:
                resolved[key] = None
            else:
                resolved[key] = val
        return {k: v for k, v in resolved.items() if v is not None}

    def ping(self, params):
        return {"message": "pong"}

    def put(self, params):
        with self.lock:
            self.mem[params["key"]] = (params["value"], False)
            return {}

    def get(self, params):
        with self.lock:
            val, found = self._lookup(params["key"])
            if found:
                return {"value": val, "found": True}
            return {"value": None, "found": False}

    def delete(self, params):
        with self.lock:
            _, existed = self._lookup(params["key"])
            self.mem[params["key"]] = ("", True)
            return {"deleted": existed}

    def flush(self, params):
        with self.lock:
            if not self.mem:
                return {}
            entries = sorted(
                [(k, v, d) for k, (v, d) in self.mem.items()],
                key=lambda x: x[0],
            )
            self._write_sst(entries)
            self.mem.clear()
            return {}

    def scan(self, params):
        with self.lock:
            start, end = params["start"], params["end"]
            live = self._merged_live()
            keys = sorted(k for k in live if start <= k < end)
            return {"entries": [{"key": k, "value": live[k]} for k in keys]}

    def compact(self, params):
        with self.lock:
            if len(self.sst_files) < 2:
                if len(self.sst_files) == 1:
                    return {}
                return {}
            resolved = {}
            for path in self.sst_files:
                for key, val, deleted in self._read_sst(path):
                    if deleted:
                        resolved[key] = (None, True)
                    else:
                        resolved[key] = (val, False)
            entries = sorted(
                [(k, v, d) for k, (v, d) in resolved.items()],
                key=lambda x: x[0],
            )
            old_files = self.sst_files[:]
            self._write_sst(entries)
            for path in old_files:
                os.remove(path)
            self.sst_files = [p for p in self.sst_files if p not in old_files]
            return {}


METHODS = {
    "ping": "ping",
    "put": "put",
    "get": "get",
    "del": "delete",
    "flush": "flush",
    "scan": "scan",
    "compact": "compact",
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
    print(f"lsm kv listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
