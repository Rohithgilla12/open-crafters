"""Reference solution for "Build your own object store" (Python). Passes all 9 stages."""

import argparse
import base64
import hashlib
import json
import os
import secrets
import socketserver
import threading


class ObjectStoreError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def body_etag(body: bytes) -> str:
    return hashlib.sha256(body).hexdigest()


class Store:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.objects = {}
        self.uploads = {}
        self.snap_path = os.path.join(data_dir, "state.json")
        self._load()

    def _load(self):
        if not os.path.exists(self.snap_path):
            return
        with open(self.snap_path) as f:
            st = json.load(f)
        for k, b64 in st.get("objects_b64", {}).items():
            self.objects[k] = base64.b64decode(b64)
        for uid, pu in st.get("uploads", {}).items():
            parts = {}
            for p in pu.get("parts", []):
                parts[p["part_number"]] = base64.b64decode(p["body_b64"])
            self.uploads[uid] = {"key": pu["key"], "parts": parts}

    def _persist(self):
        st = {"objects_b64": {}, "uploads": {}}
        for k, body in self.objects.items():
            st["objects_b64"][k] = base64.b64encode(body).decode()
        for uid, mp in self.uploads.items():
            parts = []
            for num, body in sorted(mp["parts"].items()):
                parts.append({"part_number": num, "body_b64": base64.b64encode(body).decode()})
            st["uploads"][uid] = {"key": mp["key"], "parts": parts}
        tmp = self.snap_path + ".tmp"
        with open(tmp, "w") as f:
            json.dump(st, f)
        os.replace(tmp, self.snap_path)

    def put(self, key, body):
        with self.lock:
            raw = body.encode()
            self.objects[key] = raw
            self._persist()
            return {"etag": body_etag(raw)}

    def get(self, key):
        with self.lock:
            raw = self.objects.get(key)
            if raw is None:
                raise ObjectStoreError("NOT_FOUND", f"no such key {key!r}")
            return {"found": True, "body": raw.decode(), "etag": body_etag(raw), "size": len(raw)}

    def head(self, key):
        with self.lock:
            raw = self.objects.get(key)
            if raw is None:
                raise ObjectStoreError("NOT_FOUND", f"no such key {key!r}")
            return {"found": True, "etag": body_etag(raw), "size": len(raw)}

    def delete(self, key):
        with self.lock:
            if key not in self.objects:
                return {"deleted": False}
            del self.objects[key]
            self._persist()
            return {"deleted": True}

    def list_(self, prefix):
        with self.lock:
            keys = sorted(k for k in self.objects if k.startswith(prefix))
            return {"keys": keys}

    def create_multipart(self, key):
        with self.lock:
            uid = secrets.token_hex(16)
            self.uploads[uid] = {"key": key, "parts": {}}
            self._persist()
            return {"upload_id": uid}

    def upload_part(self, upload_id, part_number, body):
        with self.lock:
            mp = self.uploads.get(upload_id)
            if mp is None:
                raise ObjectStoreError("NO_SUCH_UPLOAD", f"no upload {upload_id!r}")
            raw = body.encode()
            mp["parts"][part_number] = raw
            self._persist()
            return {"etag": body_etag(raw)}

    def complete_multipart(self, upload_id, parts):
        with self.lock:
            mp = self.uploads.get(upload_id)
            if mp is None:
                raise ObjectStoreError("NO_SUCH_UPLOAD", f"no upload {upload_id!r}")
            if not parts:
                raise ObjectStoreError("INVALID_PART", "no parts provided")
            prev = 0
            assembled = bytearray()
            for i, p in enumerate(parts):
                num = p["part_number"]
                if i > 0 and num <= prev:
                    raise ObjectStoreError("INVALID_PART", "parts must be in ascending part_number order")
                raw = mp["parts"].get(num)
                if raw is None:
                    raise ObjectStoreError("INVALID_PART", f"missing part {num}")
                etag = body_etag(raw)
                if etag != p["etag"]:
                    raise ObjectStoreError("INVALID_PART", f"etag mismatch for part {num}")
                assembled.extend(raw)
                prev = num
            del self.uploads[upload_id]
            self.objects[mp["key"]] = bytes(assembled)
            self._persist()
            return {"etag": body_etag(bytes(assembled))}

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "put":
            return self.put(params["key"], params["body"])
        if method == "get":
            return self.get(params["key"])
        if method == "head":
            return self.head(params["key"])
        if method == "delete":
            return self.delete(params["key"])
        if method == "list":
            return self.list_(params.get("prefix", ""))
        if method == "create_multipart":
            return self.create_multipart(params["key"])
        if method == "upload_part":
            return self.upload_part(params["upload_id"], params["part_number"], params["body"])
        if method == "complete_multipart":
            return self.complete_multipart(params["upload_id"], params["parts"])
        raise ObjectStoreError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            try:
                result = self.server.store.handle(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except ObjectStoreError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(resp).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=True)
    args = parser.parse_args()
    os.makedirs(args.data_dir, exist_ok=True)
    server = Server(("127.0.0.1", args.port), Handler)
    server.store = Store(args.data_dir)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
