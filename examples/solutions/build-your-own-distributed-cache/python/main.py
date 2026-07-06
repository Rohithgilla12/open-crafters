"""Reference solution for "Build your own distributed cache" (Python). Passes all 9 stages."""

import argparse
import json
import socketserver
import threading
import time
from collections import OrderedDict


class CacheError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Cache:
    def __init__(self):
        self.lock = threading.Lock()
        self.max_keys = 0
        self.data: OrderedDict[str, dict] = OrderedDict()

    def _now(self):
        return int(time.time() * 1000)

    def _expired(self, ent, now):
        return ent["expires_at"] > 0 and now >= ent["expires_at"]

    def _remove(self, key):
        self.data.pop(key, None)

    def _touch(self, key):
        self.data.move_to_end(key)

    def _get(self, key):
        now = self._now()
        ent = self.data.get(key)
        if ent is None or self._expired(ent, now):
            if ent is not None:
                self._remove(key)
            return None
        self._touch(key)
        return ent

    def _evict(self):
        while self.max_keys > 0 and len(self.data) >= self.max_keys:
            self.data.popitem(last=False)

    def configure(self, max_keys):
        if max_keys < 1:
            raise CacheError("INVALID_PARAMS", "max_keys >= 1")
        with self.lock:
            self.max_keys = max_keys

    def set(self, key, value, ttl_ms=0):
        if not key or value is None or value == "":
            raise CacheError("INVALID_PARAMS", "key and value required")
        with self.lock:
            now = self._now()
            expires = now + ttl_ms if ttl_ms > 0 else 0
            ent = self.data.get(key)
            if ent is not None and not self._expired(ent, now):
                ent["value"] = value
                ent["version"] += 1
                ent["expires_at"] = expires
                self._touch(key)
                return ent["version"]
            if ent is not None:
                self._remove(key)
            self._evict()
            self.data[key] = {"value": value, "version": 1, "expires_at": expires}
            self._touch(key)
            return 1

    def get(self, key):
        if not key:
            raise CacheError("INVALID_PARAMS", "key required")
        with self.lock:
            ent = self._get(key)
            if ent is None:
                return {"hit": False}
            return {"hit": True, "value": ent["value"], "version": ent["version"]}

    def delete(self, key):
        if not key:
            raise CacheError("INVALID_PARAMS", "key required")
        with self.lock:
            ent = self._get(key)
            if ent is None:
                return {"deleted": False}
            self._remove(key)
            return {"deleted": True}

    def setnx(self, key, value, ttl_ms=0):
        if not key or value is None or value == "":
            raise CacheError("INVALID_PARAMS", "key and value required")
        with self.lock:
            now = self._now()
            ent = self.data.get(key)
            if ent is not None and not self._expired(ent, now):
                return {"stored": False}
            if ent is not None:
                self._remove(key)
            self._evict()
            expires = now + ttl_ms if ttl_ms > 0 else 0
            self.data[key] = {"value": value, "version": 1, "expires_at": expires}
            self._touch(key)
            return {"stored": True, "version": 1}

    def cas(self, key, expected_version, value, ttl_ms=0):
        if not key or value is None or value == "":
            raise CacheError("INVALID_PARAMS", "cas fields required")
        with self.lock:
            ent = self._get(key)
            if ent is None or ent["version"] != expected_version:
                return {"swapped": False}
            ent["value"] = value
            ent["version"] += 1
            now = self._now()
            ent["expires_at"] = now + ttl_ms if ttl_ms > 0 else 0
            return {"swapped": True, "version": ent["version"]}

    def mget(self, keys):
        if not keys or len(keys) > 50:
            raise CacheError("INVALID_PARAMS", "1..50 keys required")
        with self.lock:
            entries = []
            for key in keys:
                ent = self._get(key)
                if ent is None:
                    entries.append({"key": key, "hit": False})
                else:
                    entries.append({"key": key, "hit": True, "value": ent["value"], "version": ent["version"]})
            return {"entries": entries}


CACHE = Cache()


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            method = req.get("method")
            params = req.get("params") or {}
            try:
                if method == "ping":
                    result = {"message": "pong"}
                elif method == "configure":
                    CACHE.configure(params.get("max_keys", 0))
                    result = {}
                elif method == "set":
                    ver = CACHE.set(params.get("key"), params.get("value"), params.get("ttl_ms", 0))
                    result = {"version": ver}
                elif method == "get":
                    result = CACHE.get(params.get("key"))
                elif method == "delete":
                    result = CACHE.delete(params.get("key"))
                elif method == "setnx":
                    result = CACHE.setnx(params.get("key"), params.get("value"), params.get("ttl_ms", 0))
                elif method == "cas":
                    result = CACHE.cas(params.get("key"), params.get("expected_version"), params.get("value"), params.get("ttl_ms", 0))
                elif method == "mget":
                    result = CACHE.mget(params.get("keys"))
                else:
                    raise CacheError("UNKNOWN_METHOD", f"unknown method {method!r}")
                resp = {"id": req.get("id"), "result": result}
            except CacheError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(resp).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--data-dir", required=False)
    args = p.parse_args()
    Server(("127.0.0.1", args.port), Handler).serve_forever()


if __name__ == "__main__":
    main()
