"""Reference solution for the open-crafters "Build your own rate limiter" challenge.

Keyed limiters with three algorithms (fixed window, token bucket, sliding
window), atomic admission under concurrency, and crash-durable state.
Passes all 9 stages.
"""

import argparse
import json
import math
import os
import socketserver
import threading
import time


class RateLimiterError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def now_ms():
    return int(time.time() * 1000)


class Engine:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.state_path = os.path.join(data_dir, "state.json")
        self.limiters = {}  # key -> limiter dict
        self._load()

    # --- persistence ----------------------------------------------------

    def _load(self):
        if not os.path.exists(self.state_path):
            return
        with open(self.state_path) as f:
            data = json.load(f)
        self.limiters = data.get("limiters", {})

    def _persist(self):
        # SIGKILL (not power loss) is the threat model, so an atomic rename is
        # enough — no fsync, which keeps the hot path fast. State is tiny.
        tmp = self.state_path + ".tmp"
        with open(tmp, "w") as f:
            json.dump({"limiters": self.limiters}, f)
        os.replace(tmp, self.state_path)

    # --- refill / window bookkeeping (pure, given `now`) ----------------

    def _refill(self, lim, now):
        """Bring a limiter's accrual up to `now`. Returns available units."""
        algo = lim["algorithm"]
        if algo == "token_bucket":
            elapsed = max(0, now - lim["as_of_ms"])
            accrued = elapsed / lim["refill_interval_ms"] * lim["refill_tokens"]
            lim["tokens"] = min(lim["capacity"], lim["tokens"] + accrued)
            lim["as_of_ms"] = now
            return lim["tokens"]
        if algo == "fixed_window":
            idx = now // lim["window_ms"]
            if idx != lim["window_index"]:
                lim["window_index"] = idx
                lim["count"] = 0
            return lim["limit"] - lim["count"]
        if algo == "sliding_window":
            cutoff = now - lim["window_ms"]
            lim["log"] = [e for e in lim["log"] if e[0] > cutoff]
            used = sum(e[1] for e in lim["log"])
            return lim["limit"] - used
        raise RateLimiterError("INVALID_ALGORITHM", f"unknown algorithm {algo!r}")

    def _limit_of(self, lim):
        return lim["capacity"] if lim["algorithm"] == "token_bucket" else lim["limit"]

    def _retry_after(self, lim, now, cost, available):
        """Lower bound (ms) until a take of `cost` could succeed. 0 if it can now."""
        if available >= cost:
            return 0
        algo = lim["algorithm"]
        if algo == "token_bucket":
            deficit = cost - lim["tokens"]
            return int(math.ceil(deficit / lim["refill_tokens"] * lim["refill_interval_ms"]))
        if algo == "fixed_window":
            return int((lim["window_index"] + 1) * lim["window_ms"] - now)
        if algo == "sliding_window":
            need = cost - available  # units that must age out
            freed = 0
            for ts, c in lim["log"]:  # oldest first
                freed += c
                if freed >= need:
                    return int(ts + lim["window_ms"] - now)
            return lim["window_ms"]
        return 0

    # --- methods --------------------------------------------------------

    def ping(self, params):
        return {"message": "pong"}

    def configure(self, params):
        key = params.get("key")
        algo = params.get("algorithm")
        if key is None:
            raise RateLimiterError("INVALID_PARAMS", "configure requires key")
        if algo == "token_bucket":
            lim = {
                "algorithm": algo,
                "capacity": int(params["capacity"]),
                "refill_tokens": int(params["refill_tokens"]),
                "refill_interval_ms": int(params["refill_interval_ms"]),
                "tokens": float(int(params["capacity"])),
                "as_of_ms": now_ms(),
            }
        elif algo in ("fixed_window", "sliding_window"):
            lim = {
                "algorithm": algo,
                "limit": int(params["limit"]),
                "window_ms": int(params["window_ms"]),
            }
            if algo == "fixed_window":
                lim["window_index"] = now_ms() // lim["window_ms"]
                lim["count"] = 0
            else:
                lim["log"] = []
        elif algo is None:
            raise RateLimiterError("INVALID_PARAMS", "configure requires algorithm")
        else:
            raise RateLimiterError("INVALID_ALGORITHM", f"unknown algorithm {algo!r}")

        with self.lock:
            self.limiters[key] = lim
            self._persist()
        return {}

    def _get(self, params):
        key = params.get("key")
        lim = self.limiters.get(key)
        if lim is None:
            raise RateLimiterError("KEY_NOT_FOUND", f"no limiter for key {key!r}")
        return lim

    def take(self, params):
        cost = int(params.get("cost", 1))
        with self.lock:
            lim = self._get(params)
            now = now_ms()
            available = self._refill(lim, now)
            limit = self._limit_of(lim)
            if available >= cost:
                self._consume(lim, now, cost)
                available -= cost
                self._persist()
                return {"allowed": True, "remaining": int(math.floor(available)),
                        "limit": limit, "retry_after_ms": 0}
            return {"allowed": False, "remaining": int(math.floor(available)),
                    "limit": limit, "retry_after_ms": self._retry_after(lim, now, cost, available)}

    def _consume(self, lim, now, cost):
        algo = lim["algorithm"]
        if algo == "token_bucket":
            lim["tokens"] -= cost
        elif algo == "fixed_window":
            lim["count"] += cost
        elif algo == "sliding_window":
            lim["log"].append([now, cost])

    def peek(self, params):
        cost = int(params.get("cost", 1))
        with self.lock:
            lim = self._get(params)
            now = now_ms()
            available = self._refill(lim, now)
            return {"remaining": int(math.floor(available)),
                    "limit": self._limit_of(lim),
                    "retry_after_ms": self._retry_after(lim, now, cost, available)}


engine = None


def handle_request(method, params):
    fn = getattr(engine, method, None)
    if fn is None or method.startswith("_"):
        raise RateLimiterError("UNKNOWN_METHOD", f"unknown method {method!r}")
    return fn(params or {})


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except RateLimiterError as e:
                response = {"id": request.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(response).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    global engine
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--data-dir", required=True)
    args = parser.parse_args()
    os.makedirs(args.data_dir, exist_ok=True)
    engine = Engine(args.data_dir)
    server = Server(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
