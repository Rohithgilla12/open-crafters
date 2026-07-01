"""Reference solution for "Build your own distributed lock" (Python). Passes all 9 stages."""

import argparse
import json
import os
import socketserver
import threading
import time
import uuid


class DistLockError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def now_ms():
    return int(time.time() * 1000)


class Engine:
    def __init__(self, data_dir):
        self.state_path = os.path.join(data_dir, "state.json")
        self.lock = threading.Lock()
        self.locks = {}
        self._load()

    def _load(self):
        try:
            with open(self.state_path, encoding="utf-8") as f:
                data = json.load(f)
            self.locks = data.get("locks") or {}
        except OSError:
            pass

    def _persist(self):
        tmp = self.state_path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump({"locks": self.locks}, f)
        os.replace(tmp, self.state_path)

    @staticmethod
    def _held(state, now):
        return state is not None and state.get("expires_at_ms", 0) > now

    def _validate_acquire(self, params):
        if not params.get("name") or not params.get("holder_id") or params.get("lease_ms") is None:
            raise DistLockError("INVALID_PARAMS", "acquire requires name, holder_id, lease_ms")
        if params["lease_ms"] < 1:
            raise DistLockError("INVALID_PARAMS", "lease_ms must be >= 1")

    def _grant(self, name, holder_id, lease_ms, now):
        state = {
            "holder_id": holder_id,
            "token": uuid.uuid4().hex,
            "expires_at_ms": now + lease_ms,
        }
        self.locks[name] = state
        self._persist()
        return state

    def acquire(self, params, try_mode=False):
        self._validate_acquire(params)
        with self.lock:
            now = now_ms()
            cur = self.locks.get(params["name"])
            if self._held(cur, now):
                if try_mode:
                    return {"acquired": False}
                raise DistLockError("LOCK_HELD", f'lock {params["name"]!r} is held')
            st = self._grant(params["name"], params["holder_id"], params["lease_ms"], now)
            if try_mode:
                return {"acquired": True, "token": st["token"], "expires_at_ms": st["expires_at_ms"]}
            return {"token": st["token"], "expires_at_ms": st["expires_at_ms"]}

    def release(self, params):
        if not params.get("name") or not params.get("token"):
            raise DistLockError("INVALID_PARAMS", "release requires name and token")
        with self.lock:
            now = now_ms()
            cur = self.locks.get(params["name"])
            if not self._held(cur, now) or cur["token"] != params["token"]:
                return {"released": False}
            del self.locks[params["name"]]
            self._persist()
            return {"released": True}

    def renew(self, params):
        if not params.get("name") or not params.get("token") or params.get("lease_ms") is None:
            raise DistLockError("INVALID_PARAMS", "renew requires name, token, lease_ms")
        if params["lease_ms"] < 1:
            raise DistLockError("INVALID_PARAMS", "lease_ms must be >= 1")
        with self.lock:
            now = now_ms()
            cur = self.locks.get(params["name"])
            if not self._held(cur, now) or cur["token"] != params["token"]:
                raise DistLockError("NOT_HOLDER", "token does not match current holder")
            base = max(now, cur["expires_at_ms"])
            cur["expires_at_ms"] = base + params["lease_ms"]
            self._persist()
            return {"expires_at_ms": cur["expires_at_ms"]}

    def status(self, params):
        if not params.get("name"):
            raise DistLockError("INVALID_PARAMS", "status requires name")
        with self.lock:
            now = now_ms()
            cur = self.locks.get(params["name"])
            if not self._held(cur, now):
                return {"held": False}
            return {
                "held": True,
                "holder_id": cur["holder_id"],
                "expires_at_ms": cur["expires_at_ms"],
                "token": cur["token"],
            }

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "acquire":
            return self.acquire(params)
        if method == "try_acquire":
            return self.acquire(params, try_mode=True)
        if method == "release":
            return self.release(params)
        if method == "renew":
            return self.renew(params)
        if method == "status":
            return self.status(params)
        raise DistLockError("UNKNOWN_METHOD", f"unknown method {method!r}")


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        engine = self.server.engine
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = engine.handle(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except DistLockError as e:
                response = {"id": request.get("id"), "error": {"code": e.code, "message": e.message}}
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
    os.makedirs(args.data_dir, exist_ok=True)
    server = Server(("127.0.0.1", args.port), Handler)
    server.engine = Engine(args.data_dir)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
