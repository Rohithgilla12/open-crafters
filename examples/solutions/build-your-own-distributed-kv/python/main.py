"""Reference distributed KV gateway (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import threading
import time

RING_ID = "kv"
RAFT_SHARD = "raft-shard"
LSM_SHARD = "lsm-shard"
RING_REPLICAS = 64


class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def rpc(addr, method, params, out=None):
    host, port_s = addr.rsplit(":", 1)
    conn = socket.create_connection((host, int(port_s)), timeout=10)
    try:
        conn.sendall((json.dumps({"id": "1", "method": method, "params": params}) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = conn.recv(4096)
            if not chunk:
                raise RuntimeError(f"no response from {addr}")
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise RPCError(e["code"], e["message"])
        result = resp.get("result")
        if out is not None and result is not None:
            out.update(result)
        return result or {}
    finally:
        conn.close()


class Engine:
    def __init__(self):
        self.lock = threading.Lock()
        self.ready = False
        self.ring = os.environ.get("HASHRING_ADDR", "")
        self.lsm = os.environ.get("LSM_ADDR", "")
        self.raft = [
            os.environ.get("RAFT1_ADDR", ""),
            os.environ.get("RAFT2_ADDR", ""),
            os.environ.get("RAFT3_ADDR", ""),
        ]

    def ensure_stack(self):
        with self.lock:
            if self.ready:
                return
            if not self.ring or not self.lsm or not self.raft[0]:
                raise GWError("INTERNAL", "missing HASHRING_ADDR, LSM_ADDR, or RAFT*_ADDR")
            try:
                rpc(self.ring, "create_ring", {"ring_id": RING_ID, "replicas": RING_REPLICAS})
            except RPCError as e:
                if e.code != "RING_EXISTS":
                    raise
            for node in (RAFT_SHARD, LSM_SHARD):
                try:
                    rpc(self.ring, "add_node", {"ring_id": RING_ID, "node_id": node})
                except RPCError as e:
                    if e.code != "NODE_EXISTS":
                        raise
            self.ready = True

    def lookup_node(self, key):
        out = {}
        rpc(self.ring, "lookup", {"ring_id": RING_ID, "key": key}, out)
        return out["node_id"]

    def raft_call(self, method, params, out=None):
        # Raft election can take a beat after compose boot; retry NOT_LEADER.
        deadline = time.monotonic() + 5.0
        last = None
        while True:
            for addr in self.raft:
                if not addr:
                    continue
                try:
                    return rpc(addr, method, params, out)
                except RPCError as e:
                    if e.code == "NOT_LEADER":
                        last = e
                        continue
                    raise
            if time.monotonic() >= deadline:
                break
            time.sleep(0.05)
        if last:
            raise last
        raise GWError("NOT_LEADER", "no raft leader available")

    def put(self, key, value):
        node = self.lookup_node(key)
        if node == RAFT_SHARD:
            self.raft_call("set", {"key": key, "value": value})
        elif node == LSM_SHARD:
            rpc(self.lsm, "put", {"key": key, "value": value})
        else:
            raise GWError("INTERNAL", f"unknown shard {node}")

    def get(self, key):
        node = self.lookup_node(key)
        if node == RAFT_SHARD:
            out = {}
            self.raft_call("get", {"key": key}, out)
            if not out.get("found"):
                return False, ""
            val = out.get("value")
            return True, val if isinstance(val, str) else str(val)
        if node == LSM_SHARD:
            out = {}
            rpc(self.lsm, "get", {"key": key}, out)
            return out.get("found", False), out.get("value", "")
        raise GWError("INTERNAL", f"unknown shard {node}")

    def delete(self, key):
        node = self.lookup_node(key)
        if node != LSM_SHARD:
            raise GWError("UNSUPPORTED", "delete only supported on LSM shard keys")
        out = {}
        rpc(self.lsm, "del", {"key": key}, out)
        return out.get("deleted", False)

    def handle(self, method, params):
        if method == "ping":
            self.ensure_stack()
            return {"message": "pong"}
        self.ensure_stack()
        if method == "put":
            key, val = params.get("key"), params.get("value")
            if not key or val is None:
                raise GWError("INVALID_PARAMS", "put requires key and value")
            try:
                self.put(key, val)
            except RPCError as e:
                raise GWError(e.code, e.message) from e
            return {}
        if method == "get":
            key = params.get("key")
            if not key:
                raise GWError("INVALID_PARAMS", "get requires key")
            try:
                found, val = self.get(key)
            except RPCError as e:
                raise GWError(e.code, e.message) from e
            if not found:
                return {"found": False, "value": None}
            return {"found": True, "value": val}
        if method == "delete":
            key = params.get("key")
            if not key:
                raise GWError("INVALID_PARAMS", "delete requires key")
            try:
                deleted = self.delete(key)
            except RPCError as e:
                raise GWError(e.code, e.message) from e
            except GWError:
                raise
            return {"deleted": deleted}
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
