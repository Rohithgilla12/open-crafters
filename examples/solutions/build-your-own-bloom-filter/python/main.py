"""Reference solution for "Build your own bloom filter" (Python). Passes all 9 stages."""

import argparse
import json
import socketserver
import threading

FNV_OFFSET64 = 14695981039346656037
FNV_PRIME64 = 1099511628211


class BloomFilterError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def fnv1a64(data: bytes) -> int:
    h = FNV_OFFSET64
    for b in data:
        h ^= b
        h = (h * FNV_PRIME64) & 0xFFFFFFFFFFFFFFFF
    return h


def hash_positions(item: str, m: int, k: int) -> list[int]:
    item_bytes = item.encode("utf-8")
    h1 = fnv1a64(item_bytes)
    h2 = fnv1a64(item_bytes + b"\x01")
    return [int((h1 + i * h2) % m) for i in range(k)]


class BloomFilter:
    def __init__(self, m: int, k: int):
        self.m = m
        self.k = k
        self.bits = bytearray((m + 7) // 8)

    def _set(self, i: int) -> None:
        self.bits[i // 8] |= 1 << (i % 8)

    def _get(self, i: int) -> bool:
        return bool(self.bits[i // 8] & (1 << (i % 8)))

    def add(self, item: str) -> None:
        for pos in hash_positions(item, self.m, self.k):
            self._set(pos)

    def contains(self, item: str) -> bool:
        return all(self._get(pos) for pos in hash_positions(item, self.m, self.k))


class Engine:
    def __init__(self):
        self.lock = threading.Lock()
        self.filters: dict[str, BloomFilter] = {}

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "create":
            fid = params.get("filter_id")
            m = params.get("m")
            k = params.get("k")
            if not fid or m is None or k is None or m < 8 or k < 1:
                raise BloomFilterError("INVALID_PARAMS", "create requires filter_id, m>=8, k>=1")
            with self.lock:
                if fid in self.filters:
                    raise BloomFilterError("FILTER_EXISTS", f"filter {fid!r} already exists")
                self.filters[fid] = BloomFilter(m, k)
            return {}
        if method == "add":
            fid = params.get("filter_id")
            item = params.get("item")
            if not fid or not item:
                raise BloomFilterError("INVALID_PARAMS", "add requires filter_id and item")
            with self.lock:
                bf = self.filters.get(fid)
                if bf is None:
                    raise BloomFilterError("FILTER_NOT_FOUND", f"no filter {fid!r}")
                bf.add(item)
            return {}
        if method == "contains":
            fid = params.get("filter_id")
            item = params.get("item")
            if not fid or not item:
                raise BloomFilterError("INVALID_PARAMS", "contains requires filter_id and item")
            with self.lock:
                bf = self.filters.get(fid)
                if bf is None:
                    raise BloomFilterError("FILTER_NOT_FOUND", f"no filter {fid!r}")
                return {"maybe_present": bf.contains(item)}
        if method == "delete_filter":
            fid = params.get("filter_id")
            if not fid:
                raise BloomFilterError("INVALID_PARAMS", "delete_filter requires filter_id")
            with self.lock:
                if fid not in self.filters:
                    return {"deleted": False}
                del self.filters[fid]
                return {"deleted": True}
        raise BloomFilterError("UNKNOWN_METHOD", f"unknown method {method!r}")


ENGINE = Engine()


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = ENGINE.handle(request.get("method"), request.get("params") or {})
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
    parser.add_argument("--data-dir", required=False)
    args = parser.parse_args()
    server = Server(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
