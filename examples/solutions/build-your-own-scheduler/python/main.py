"""Reference solution for the open-crafters "Build your own scheduler" challenge.

Durable job scheduler with delayed/recurring jobs, leases, and retries.
Passes all 9 stages.
"""

import argparse
import json
import os
import socketserver
import threading
import time
import uuid


class SchedulerError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Engine:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.state_path = os.path.join(data_dir, "state.json")
        self.jobs = {}  # job_id -> dict
        self.leases = {}  # token -> job_id
        self._load()

    def _load(self):
        if not os.path.exists(self.state_path):
            return
        with open(self.state_path) as f:
            data = json.load(f)
        for job in data.get("jobs", []):
            self.jobs[job["job_id"]] = job

    def _persist(self):
        os.makedirs(os.path.dirname(self.state_path) or ".", exist_ok=True)
        jobs = []
        for job in self.jobs.values():
            j = {k: v for k, v in job.items() if not k.startswith("_")}
            jobs.append(j)
        tmp = self.state_path + ".tmp"
        with open(tmp, "w") as f:
            json.dump({"jobs": jobs}, f)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, self.state_path)

    def _now_ms(self):
        return int(time.time() * 1000)

    def _release_expired_leases(self):
        now = self._now_ms()
        for job in self.jobs.values():
            if job["status"] == "leased" and job.get("lease_expires_at_ms", 0) <= now:
                job["status"] = "pending"
                job.pop("lease_token", None)
                job.pop("lease_expires_at_ms", None)

    def _pollable(self, job):
        if job["status"] == "cancelled" or job["status"] in ("completed", "failed"):
            return False
        now = self._now_ms()
        if job["run_at_ms"] > now:
            return False
        if job["status"] == "leased":
            return job.get("lease_expires_at_ms", 0) <= now
        return job["status"] == "pending"

    def ping(self, params):
        return {"message": "pong"}

    def schedule(self, params):
        payload = params.get("payload")
        now = self._now_ms()
        if "delay_ms" in params:
            run_at = now + int(params["delay_ms"])
        elif "run_at_ms" in params:
            run_at = int(params["run_at_ms"])
        else:
            raise SchedulerError("INVALID_PARAMS", "schedule requires delay_ms or run_at_ms")

        retry = params.get("retry_policy") or {}
        max_attempts = int(retry.get("maximum_attempts", 1))
        retry_delay = int(retry.get("retry_delay_ms", 0))

        job_id = "j-" + uuid.uuid4().hex[:12]
        job = {
            "job_id": job_id,
            "payload": payload,
            "run_at_ms": run_at,
            "status": "pending",
            "attempt": 1,
            "lease_ms": int(params.get("lease_ms", 3000)),
            "max_attempts": max_attempts,
            "retry_delay_ms": retry_delay,
            "interval_ms": params.get("interval_ms"),
            "result": None,
            "error": None,
        }
        with self.lock:
            self.jobs[job_id] = job
            self._persist()
        return {"job_id": job_id}

    def poll(self, params):
        with self.lock:
            self._release_expired_leases()
            candidates = [j for j in self.jobs.values() if self._pollable(j)]
            if not candidates:
                return {"job": None}
            job = min(candidates, key=lambda j: j["run_at_ms"])
            token = uuid.uuid4().hex
            job["status"] = "leased"
            job["lease_token"] = token
            job["lease_expires_at_ms"] = self._now_ms() + job["lease_ms"]
            self.leases[token] = job["job_id"]
            return {
                "job": {
                    "job_id": job["job_id"],
                    "payload": job["payload"],
                    "attempt": job["attempt"],
                    "lease_token": token,
                }
            }

    def _job_by_token(self, token):
        job_id = self.leases.get(token)
        if not job_id:
            raise SchedulerError("LEASE_NOT_FOUND", f"unknown lease token {token!r}")
        job = self.jobs.get(job_id)
        if not job or job.get("lease_token") != token:
            raise SchedulerError("LEASE_NOT_FOUND", "lease expired or invalid")
        if job.get("lease_expires_at_ms", 0) <= self._now_ms():
            raise SchedulerError("LEASE_NOT_FOUND", "lease expired")
        return job

    def complete(self, params):
        token = params["lease_token"]
        result = params.get("result")
        with self.lock:
            job = self._job_by_token(token)
            job["status"] = "completed"
            job["result"] = result
            job.pop("lease_token", None)
            job.pop("lease_expires_at_ms", None)
            self.leases.pop(token, None)
            interval = job.get("interval_ms")
            if interval:
                self._spawn_next(job, interval)
            self._persist()
        return {}

    def _spawn_next(self, parent, interval_ms):
        job_id = "j-" + uuid.uuid4().hex[:12]
        now = self._now_ms()
        self.jobs[job_id] = {
            "job_id": job_id,
            "payload": parent["payload"],
            "run_at_ms": now + int(interval_ms),
            "status": "pending",
            "attempt": 1,
            "lease_ms": parent["lease_ms"],
            "max_attempts": parent["max_attempts"],
            "retry_delay_ms": parent["retry_delay_ms"],
            "interval_ms": parent.get("interval_ms"),
            "result": None,
            "error": None,
        }

    def fail(self, params):
        token = params["lease_token"]
        err_msg = params.get("error", "failed")
        with self.lock:
            job = self._job_by_token(token)
            job.pop("lease_token", None)
            job.pop("lease_expires_at_ms", None)
            self.leases.pop(token, None)
            if job["attempt"] < job["max_attempts"]:
                job["attempt"] += 1
                job["status"] = "pending"
                job["run_at_ms"] = self._now_ms() + job["retry_delay_ms"]
            else:
                job["status"] = "failed"
                job["error"] = err_msg
            self._persist()
        return {}

    def cancel(self, params):
        job_id = params["job_id"]
        with self.lock:
            job = self.jobs.get(job_id)
            if job is None:
                raise SchedulerError("JOB_NOT_FOUND", f"no job {job_id!r}")
            if job["status"] != "pending":
                return {"cancelled": False}
            job["status"] = "cancelled"
            self._persist()
            return {"cancelled": True}

    def get_job(self, params):
        job_id = params["job_id"]
        with self.lock:
            self._release_expired_leases()
            job = self.jobs.get(job_id)
            if job is None:
                raise SchedulerError("JOB_NOT_FOUND", f"no job {job_id!r}")
            return {
                "job_id": job["job_id"],
                "status": job["status"],
                "payload": job["payload"],
                "run_at_ms": job["run_at_ms"],
                "attempt": job["attempt"],
                "result": job.get("result"),
                "error": job.get("error"),
            }


engine = None


def handle_request(method, params):
    fn = getattr(engine, method, None)
    if fn is None:
        raise SchedulerError("UNKNOWN_METHOD", f"unknown method {method!r}")
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
            except SchedulerError as e:
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
    engine = Engine(args.data_dir)
    server = Server(("127.0.0.1", args.port), Handler)
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
