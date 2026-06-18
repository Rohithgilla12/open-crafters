"""Reference solution for the open-crafters "Build your own message queue".

A durable broker with at-least-once delivery:
  - send/receive/ack/nack with per-receive visibility timeouts
  - receipt fencing: a receipt is valid for exactly one delivery
  - dead-letter queues after max_receives failed deliveries
  - durability: un-acked messages and queue config survive SIGKILL; acked
    messages stay gone (state snapshotted atomically to --data-dir)

The on-disk format is deliberately not part of the contract (that is the WAL
challenge's job) — here durability is graded purely behaviorally. Passes all
9 stages.
"""

import argparse
import json
import os
import socketserver
import threading
import time
import uuid


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Message:
    __slots__ = ("id", "body", "seq", "receives", "inflight", "invisible_until", "receipt")

    def __init__(self, id, body, seq, receives=0):
        self.id = id
        self.body = body
        self.seq = seq
        self.receives = receives
        self.inflight = False
        self.invisible_until = 0.0
        self.receipt = None


class Queue:
    def __init__(self):
        self.messages = {}  # id -> Message
        self.max_receives = None
        self.dead_letter_queue = None


class Broker:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.snapshot_path = os.path.join(data_dir, "state.json")
        self.queues = {}  # name -> Queue
        self.seq = 0
        self._recover()

    # ----- durability -----

    def _recover(self):
        if not os.path.exists(self.snapshot_path):
            return
        with open(self.snapshot_path) as f:
            state = json.load(f)
        self.seq = state.get("seq", 0)
        for name, q in state.get("queues", {}).items():
            queue = Queue()
            queue.max_receives = q.get("max_receives")
            queue.dead_letter_queue = q.get("dead_letter_queue")
            for m in q.get("messages", []):
                # Everything un-acked comes back visible; in-flight bookkeeping
                # does not survive a crash.
                queue.messages[m["id"]] = Message(m["id"], m["body"], m["seq"], m.get("receives", 0))
            self.queues[name] = queue

    def _persist(self):
        state = {"seq": self.seq, "queues": {}}
        for name, q in self.queues.items():
            state["queues"][name] = {
                "max_receives": q.max_receives,
                "dead_letter_queue": q.dead_letter_queue,
                "messages": [
                    {"id": m.id, "body": m.body, "seq": m.seq, "receives": m.receives}
                    for m in q.messages.values()
                ],
            }
        tmp = self.snapshot_path + ".tmp"
        with open(tmp, "w") as f:
            json.dump(state, f)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, self.snapshot_path)

    # ----- helpers -----

    def _queue(self, name):
        if name not in self.queues:
            self.queues[name] = Queue()
        return self.queues[name]

    def _next_seq(self):
        self.seq += 1
        return self.seq

    def _expire(self, queue, now):
        """Return in-flight messages whose visibility has lapsed to visible,
        or dead-letter them. Returns True if durable state changed."""
        changed = False
        for m in list(queue.messages.values()):
            if m.inflight and m.invisible_until <= now:
                if self._maybe_dead_letter(queue, m):
                    changed = True
                else:
                    m.inflight = False
                    m.receipt = None
        return changed

    def _maybe_dead_letter(self, queue, m):
        """If the queue has a policy and m has hit it, move m to the DLQ and
        return True; otherwise leave it for normal redelivery and return
        False."""
        if queue.max_receives is None or m.receives < queue.max_receives:
            return False
        del queue.messages[m.id]
        dlq = self._queue(queue.dead_letter_queue)
        moved = Message(m.id, m.body, self._next_seq(), receives=0)
        dlq.messages[m.id] = moved
        return True

    # ----- RPC methods -----

    def ping(self, params):
        return {"message": "pong"}

    def send(self, params):
        with self.lock:
            queue = self._queue(params["queue"])
            mid = uuid.uuid4().hex
            queue.messages[mid] = Message(mid, params["body"], self._next_seq())
            self._persist()
            return {"id": mid}

    def receive(self, params):
        timeout_ms = params.get("visibility_timeout_ms", 30000)
        with self.lock:
            queue = self._queue(params["queue"])
            now = time.monotonic()
            if self._expire(queue, now):
                self._persist()
            visible = [m for m in queue.messages.values() if not m.inflight]
            if not visible:
                return {"message": None}
            m = min(visible, key=lambda x: x.seq)
            m.receives += 1
            m.inflight = True
            m.invisible_until = now + timeout_ms / 1000.0
            m.receipt = uuid.uuid4().hex
            return {"message": {"id": m.id, "body": m.body, "receipt": m.receipt, "receives": m.receives}}

    def _find_inflight(self, queue, receipt):
        for m in queue.messages.values():
            if m.inflight and m.receipt == receipt:
                return m
        return None

    def ack(self, params):
        with self.lock:
            queue = self._queue(params["queue"])
            m = self._find_inflight(queue, params["receipt"])
            if m is None:
                return {"acked": False}
            del queue.messages[m.id]
            self._persist()
            return {"acked": True}

    def nack(self, params):
        with self.lock:
            queue = self._queue(params["queue"])
            m = self._find_inflight(queue, params["receipt"])
            if m is None:
                return {"nacked": False}
            if self._maybe_dead_letter(queue, m):
                self._persist()
            else:
                m.inflight = False
                m.invisible_until = 0.0
                m.receipt = None
            return {"nacked": True}

    def stats(self, params):
        with self.lock:
            queue = self.queues.get(params["queue"])
            if queue is None:
                return {"visible": 0, "inflight": 0}
            now = time.monotonic()
            if self._expire(queue, now):
                self._persist()
            visible = sum(1 for m in queue.messages.values() if not m.inflight)
            inflight = sum(1 for m in queue.messages.values() if m.inflight)
            return {"visible": visible, "inflight": inflight}

    def configure(self, params):
        with self.lock:
            queue = self._queue(params["queue"])
            queue.max_receives = params["max_receives"]
            queue.dead_letter_queue = params["dead_letter_queue"]
            self._persist()
            return {}


METHODS = {
    "ping": "ping",
    "send": "send",
    "receive": "receive",
    "ack": "ack",
    "nack": "nack",
    "stats": "stats",
    "configure": "configure",
}


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        broker = self.server.broker
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
                    raise RPCError("UNKNOWN_METHOD", f"unknown method {request.get('method')!r}")
                result = getattr(broker, method)(request.get("params") or {})
                response = {"id": request_id, "result": result}
            except RPCError as e:
                response = {"id": request_id, "error": {"code": e.code, "message": e.message}}
            except Exception as e:
                response = {"id": request_id, "error": {"code": "BAD_REQUEST", "message": str(e)}}
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
    server.broker = Broker(args.data_dir)
    print(f"message broker listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
