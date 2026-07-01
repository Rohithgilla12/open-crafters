"""Reference solution for "Build your own hash ring" (Python). Passes all 9 stages."""

import argparse
import json
import socketserver
import threading

FNV_OFFSET64 = 14695981039346656037
FNV_PRIME64 = 1099511628211


class HashRingError(Exception):
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


def hash_key(key: str) -> int:
    return fnv1a64(key.encode("utf-8"))


def vnode_position(node_id: str, replica: int) -> int:
    return fnv1a64(f"{node_id}#{replica}".encode("utf-8"))


class Ring:
    def __init__(self, replicas: int):
        self.replicas = replicas
        self.nodes: set[str] = set()

    def sorted_nodes(self) -> list[str]:
        return sorted(self.nodes)

    def lookup(self, key: str) -> str:
        if not self.nodes:
            raise HashRingError("NO_NODES", "ring has no nodes")
        vnodes = []
        for node_id in self.sorted_nodes():
            for i in range(self.replicas):
                vnodes.append((vnode_position(node_id, i), node_id))
        vnodes.sort(key=lambda x: (x[0], x[1]))
        h = hash_key(key)
        for pos, node_id in vnodes:
            if pos >= h:
                return node_id
        return vnodes[0][1]


class Engine:
    def __init__(self):
        self.lock = threading.Lock()
        self.rings: dict[str, Ring] = {}

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "create_ring":
            ring_id = params.get("ring_id")
            replicas = params.get("replicas")
            if not ring_id or replicas is None or replicas < 1:
                raise HashRingError("INVALID_PARAMS", "create_ring requires ring_id and replicas>=1")
            with self.lock:
                if ring_id in self.rings:
                    raise HashRingError("RING_EXISTS", f"ring {ring_id!r} already exists")
                self.rings[ring_id] = Ring(replicas)
            return {}
        if method == "add_node":
            ring_id = params.get("ring_id")
            node_id = params.get("node_id")
            if not ring_id or not node_id:
                raise HashRingError("INVALID_PARAMS", "add_node requires ring_id and node_id")
            with self.lock:
                ring = self.rings.get(ring_id)
                if ring is None:
                    raise HashRingError("RING_NOT_FOUND", f"no ring {ring_id!r}")
                if node_id in ring.nodes:
                    raise HashRingError("NODE_EXISTS", f"node {node_id!r} already on ring")
                ring.nodes.add(node_id)
            return {}
        if method == "remove_node":
            ring_id = params.get("ring_id")
            node_id = params.get("node_id")
            if not ring_id or not node_id:
                raise HashRingError("INVALID_PARAMS", "remove_node requires ring_id and node_id")
            with self.lock:
                ring = self.rings.get(ring_id)
                if ring is None:
                    raise HashRingError("RING_NOT_FOUND", f"no ring {ring_id!r}")
                if node_id not in ring.nodes:
                    return {"removed": False}
                ring.nodes.remove(node_id)
            return {"removed": True}
        if method == "lookup":
            ring_id = params.get("ring_id")
            key = params.get("key")
            if not ring_id or not key:
                raise HashRingError("INVALID_PARAMS", "lookup requires ring_id and key")
            with self.lock:
                ring = self.rings.get(ring_id)
                if ring is None:
                    raise HashRingError("RING_NOT_FOUND", f"no ring {ring_id!r}")
                return {"node_id": ring.lookup(key)}
        if method == "list_nodes":
            ring_id = params.get("ring_id")
            if not ring_id:
                raise HashRingError("INVALID_PARAMS", "list_nodes requires ring_id")
            with self.lock:
                ring = self.rings.get(ring_id)
                if ring is None:
                    raise HashRingError("RING_NOT_FOUND", f"no ring {ring_id!r}")
                return {"nodes": ring.sorted_nodes()}
        raise HashRingError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = engine.handle(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except HashRingError as e:
                response = {"id": request.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(response).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


engine = Engine()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=False)
    args = parser.parse_args()
    _ = args.data_dir
    server = Server(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
