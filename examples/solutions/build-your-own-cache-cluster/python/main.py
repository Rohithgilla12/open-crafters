"""Reference cache cluster gateway (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import threading

RING_ID = "cache"
NODE1, NODE2 = "node1", "node2"
FILTER_ID = "keys"


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def rpc(addr, method, params):
    host, port_s = addr.rsplit(":", 1)
    conn = socket.create_connection((host, int(port_s)), timeout=10)
    try:
        conn.sendall((json.dumps({"id": "1", "method": method, "params": params}) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = conn.recv(4096)
            if not chunk:
                break
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise GWError(e["code"], e["message"])
        return resp.get("result", {})
    finally:
        conn.close()


class Engine:
    def __init__(self):
        self.lock = threading.Lock()
        self.ready = False
        self.ring = os.environ.get("HASHRING_ADDR", "")
        self.bloom = os.environ.get("BLOOM_ADDR", "")
        self.limiter = os.environ.get("LIMITER_ADDR", "")
        self.cache1 = os.environ.get("CACHE_NODE1_ADDR", "")
        self.cache2 = os.environ.get("CACHE_NODE2_ADDR", "")

    def ensure_stack(self):
        with self.lock:
            if self.ready:
                return
            try:
                rpc(self.ring, "create_ring", {"ring_id": RING_ID, "replicas": 64})
            except GWError as e:
                if e.code != "RING_EXISTS":
                    raise
            for n in (NODE1, NODE2):
                try:
                    rpc(self.ring, "add_node", {"ring_id": RING_ID, "node_id": n})
                except GWError as e:
                    if e.code != "NODE_EXISTS":
                        raise
            try:
                rpc(self.bloom, "create", {"filter_id": FILTER_ID, "m": 8192, "k": 4})
            except GWError as e:
                if e.code != "FILTER_EXISTS":
                    raise
            for addr in (self.cache1, self.cache2):
                rpc(addr, "configure", {"max_keys": 4096})
            self.ready = True

    def cache_addr(self, node_id):
        if node_id == NODE1:
            return self.cache1
        if node_id == NODE2:
            return self.cache2
        raise GWError("INTERNAL", f"unknown node {node_id}")

    def lookup(self, key):
        return rpc(self.ring, "lookup", {"ring_id": RING_ID, "key": key})["node_id"]

    def admit(self, key):
        rl_key = f"rl:{key}"
        try:
            res = rpc(self.limiter, "take", {"key": rl_key, "cost": 1})
        except GWError as e:
            if e.code != "KEY_NOT_FOUND":
                raise
            rpc(self.limiter, "configure", {
                "key": rl_key, "algorithm": "token_bucket",
                "capacity": 100, "refill_tokens": 100, "refill_interval_ms": 1000,
            })
            res = rpc(self.limiter, "take", {"key": rl_key, "cost": 1})
        if not res.get("allowed"):
            raise GWError("RATE_LIMITED", "rate limit exceeded")

    def bloom_maybe(self, key):
        return rpc(self.bloom, "contains", {"filter_id": FILTER_ID, "item": key}).get("maybe_present", False)

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        self.ensure_stack()
        if method == "set":
            key, val = params.get("key"), params.get("value")
            if not key or val is None:
                raise GWError("INVALID_PARAMS", "set requires key and value")
            self.admit(key)
            node = self.lookup(key)
            p = {"key": key, "value": val}
            if params.get("ttl_ms", 0) > 0:
                p["ttl_ms"] = params["ttl_ms"]
            ver = rpc(self.cache_addr(node), "set", p)["version"]
            rpc(self.bloom, "add", {"filter_id": FILTER_ID, "item": key})
            return {"version": ver}
        if method == "get":
            key = params.get("key")
            if not key:
                raise GWError("INVALID_PARAMS", "get requires key")
            if not self.bloom_maybe(key):
                return {"hit": False}
            self.admit(key)
            node = self.lookup(key)
            res = rpc(self.cache_addr(node), "get", {"key": key})
            if not res.get("hit"):
                return {"hit": False}
            return {"hit": True, "value": res["value"], "version": res["version"]}
        if method == "delete":
            key = params.get("key")
            if not key:
                raise GWError("INVALID_PARAMS", "delete requires key")
            self.admit(key)
            node = self.lookup(key)
            return {"deleted": rpc(self.cache_addr(node), "delete", {"key": key}).get("deleted", False)}
        if method == "mget":
            keys = params.get("keys") or []
            if not keys:
                raise GWError("INVALID_PARAMS", "mget requires keys")
            entries = [self.handle("get", {"key": k}) | {"key": k} for k in keys]
            for e in entries:
                if "value" not in e and not e.get("hit"):
                    e.pop("version", None)
            return {"entries": entries}
        raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


ENGINE = Engine()


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            try:
                result = ENGINE.handle(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except GWError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            except Exception as e:
                resp = {"id": req.get("id"), "error": {"code": "INTERNAL", "message": str(e)}}
            self.wfile.write(json.dumps(resp).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--data-dir", default="")
    args = p.parse_args()
    with Server(("127.0.0.1", args.port), Handler) as srv:
        print(f"listening on 127.0.0.1:{args.port}", flush=True)
        srv.serve_forever()


if __name__ == "__main__":
    main()
