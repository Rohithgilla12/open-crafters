"""Reference solution for the open-crafters "Build your own Temporal" challenge.

A minimal but complete workflow engine server:
  - newline-delimited JSON over TCP (see PROTOCOL.md)
  - append-only event histories with workflow/activity task dispatch
  - activity retries with exponential backoff
  - durable timers
  - signals
  - crash-safe persistence (atomic JSON snapshot per state change)

Passes all 10 stages. Kept deliberately straightforward: one big lock, a
ticker thread for timers, full-snapshot persistence.
"""

import argparse
import json
import os
import socketserver
import threading
import time
import uuid


class EngineError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Engine:
    def __init__(self, data_dir):
        self.lock = threading.Lock()
        self.state_path = os.path.join(data_dir, "state.json")
        self.workflows = {}  # workflow_id -> dict, insertion-ordered
        # In-memory only; forgotten on restart so unfinished tasks re-deliver.
        self.wft_claims = {}  # task_token -> workflow_id
        self.activity_claims = {}  # task_token -> (workflow_id, activity_id)
        self._load()

    # ----- persistence -----

    def _load(self):
        if not os.path.exists(self.state_path):
            return
        with open(self.state_path) as f:
            snapshot = json.load(f)
        for wf in snapshot["workflows"]:
            wf["wft_token"] = None
            for act in wf["pending_activities"].values():
                act["claimed"] = False
            self.workflows[wf["workflow_id"]] = wf

    def _persist(self):
        workflows = []
        for wf in self.workflows.values():
            workflows.append({
                "workflow_id": wf["workflow_id"],
                "run_id": wf["run_id"],
                "workflow_type": wf["workflow_type"],
                "task_queue": wf["task_queue"],
                "status": wf["status"],
                "result": wf["result"],
                "error": wf["error"],
                "history": wf["history"],
                # A claimed-but-incomplete workflow task must be re-delivered
                # after a crash, so persist the claim as "needs a task".
                "needs_wft": wf["needs_wft"] or wf["wft_token"] is not None,
                "pending_activities": {
                    aid: {k: v for k, v in act.items() if k != "claimed"}
                    for aid, act in wf["pending_activities"].items()
                },
                "timers": wf["timers"],
            })
        tmp = self.state_path + ".tmp"
        with open(tmp, "w") as f:
            json.dump({"workflows": workflows}, f)
            f.flush()
            os.fsync(f.fileno())
        os.replace(tmp, self.state_path)

    # ----- helpers -----

    def _get(self, workflow_id):
        wf = self.workflows.get(workflow_id)
        if wf is None:
            raise EngineError("WORKFLOW_NOT_FOUND", f"no workflow with id {workflow_id!r}")
        return wf

    def _append(self, wf, event_type, attributes):
        wf["history"].append({
            "event_id": len(wf["history"]) + 1,
            "type": event_type,
            "attributes": attributes,
        })

    # ----- RPC methods -----

    def ping(self, params):
        return {"message": "pong"}

    def start_workflow(self, params):
        workflow_id = params["workflow_id"]
        with self.lock:
            if workflow_id in self.workflows:
                raise EngineError("WORKFLOW_ALREADY_EXISTS",
                                  f"workflow {workflow_id!r} already exists")
            wf = {
                "workflow_id": workflow_id,
                "run_id": uuid.uuid4().hex,
                "workflow_type": params["workflow_type"],
                "task_queue": params["task_queue"],
                "status": "RUNNING",
                "result": None,
                "error": None,
                "history": [],
                "needs_wft": True,
                "wft_token": None,
                "pending_activities": {},
                "timers": {},
            }
            self._append(wf, "WORKFLOW_EXECUTION_STARTED", {
                "workflow_type": params["workflow_type"],
                "input": params.get("input"),
            })
            self.workflows[workflow_id] = wf
            self._persist()
            return {"run_id": wf["run_id"]}

    def describe_workflow(self, params):
        with self.lock:
            wf = self._get(params["workflow_id"])
            return {
                "workflow_id": wf["workflow_id"],
                "run_id": wf["run_id"],
                "workflow_type": wf["workflow_type"],
                "status": wf["status"],
                "result": wf["result"],
                "error": wf["error"],
            }

    def get_history(self, params):
        with self.lock:
            wf = self._get(params["workflow_id"])
            return {"events": list(wf["history"])}

    def poll_workflow_task(self, params):
        with self.lock:
            for wf in self.workflows.values():
                if (wf["task_queue"] == params["task_queue"]
                        and wf["status"] == "RUNNING"
                        and wf["needs_wft"]
                        and wf["wft_token"] is None):
                    token = uuid.uuid4().hex
                    wf["needs_wft"] = False
                    wf["wft_token"] = token
                    self.wft_claims[token] = wf["workflow_id"]
                    return {"task": {
                        "task_token": token,
                        "workflow_id": wf["workflow_id"],
                        "run_id": wf["run_id"],
                        "workflow_type": wf["workflow_type"],
                        "history": list(wf["history"]),
                    }}
            return {"task": None}

    def complete_workflow_task(self, params):
        with self.lock:
            workflow_id = self.wft_claims.pop(params["task_token"], None)
            if workflow_id is None:
                raise EngineError("TASK_NOT_FOUND",
                                  f"no claimed workflow task with token {params['task_token']!r}")
            wf = self.workflows[workflow_id]
            wf["wft_token"] = None
            for command in params.get("commands", []):
                self._apply_command(wf, command)
            if wf["status"] != "RUNNING":
                wf["needs_wft"] = False
            self._persist()
            return {}

    def _apply_command(self, wf, command):
        ctype = command["type"]
        attrs = command.get("attributes", {})
        if ctype == "SCHEDULE_ACTIVITY":
            activity_id = attrs["activity_id"]
            self._append(wf, "ACTIVITY_TASK_SCHEDULED", {
                "activity_id": activity_id,
                "activity_type": attrs["activity_type"],
                "input": attrs.get("input"),
            })
            policy = attrs.get("retry_policy") or {}
            wf["pending_activities"][activity_id] = {
                "activity_type": attrs["activity_type"],
                "input": attrs.get("input"),
                "attempt": 1,
                "maximum_attempts": policy.get("maximum_attempts", 1),
                "initial_interval_ms": policy.get("initial_interval_ms", 1000),
                "backoff_coefficient": policy.get("backoff_coefficient", 2.0),
                "available_at": time.time(),
                "claimed": False,
            }
        elif ctype == "START_TIMER":
            self._append(wf, "TIMER_STARTED", {
                "timer_id": attrs["timer_id"],
                "duration_ms": attrs["duration_ms"],
            })
            wf["timers"][attrs["timer_id"]] = time.time() + attrs["duration_ms"] / 1000.0
        elif ctype == "COMPLETE_WORKFLOW":
            self._append(wf, "WORKFLOW_EXECUTION_COMPLETED", {"result": attrs.get("result")})
            wf["status"] = "COMPLETED"
            wf["result"] = attrs.get("result")
        elif ctype == "FAIL_WORKFLOW":
            self._append(wf, "WORKFLOW_EXECUTION_FAILED", {"error": attrs.get("error")})
            wf["status"] = "FAILED"
            wf["error"] = attrs.get("error")
        else:
            raise EngineError("UNKNOWN_COMMAND", f"unknown command type {ctype!r}")

    def poll_activity_task(self, params):
        now = time.time()
        with self.lock:
            for wf in self.workflows.values():
                if wf["task_queue"] != params["task_queue"] or wf["status"] != "RUNNING":
                    continue
                for activity_id, act in wf["pending_activities"].items():
                    if act["claimed"] or act["available_at"] > now:
                        continue
                    token = uuid.uuid4().hex
                    act["claimed"] = True
                    self.activity_claims[token] = (wf["workflow_id"], activity_id)
                    return {"task": {
                        "task_token": token,
                        "workflow_id": wf["workflow_id"],
                        "run_id": wf["run_id"],
                        "activity_id": activity_id,
                        "activity_type": act["activity_type"],
                        "input": act["input"],
                        "attempt": act["attempt"],
                    }}
            return {"task": None}

    def _take_activity_claim(self, token):
        claim = self.activity_claims.pop(token, None)
        if claim is None:
            raise EngineError("TASK_NOT_FOUND",
                              f"no claimed activity task with token {token!r}")
        workflow_id, activity_id = claim
        wf = self.workflows[workflow_id]
        return wf, activity_id

    def complete_activity_task(self, params):
        with self.lock:
            wf, activity_id = self._take_activity_claim(params["task_token"])
            del wf["pending_activities"][activity_id]
            self._append(wf, "ACTIVITY_TASK_COMPLETED", {
                "activity_id": activity_id,
                "result": params.get("result"),
            })
            wf["needs_wft"] = True
            self._persist()
            return {}

    def fail_activity_task(self, params):
        with self.lock:
            wf, activity_id = self._take_activity_claim(params["task_token"])
            act = wf["pending_activities"][activity_id]
            attempt = act["attempt"]
            if attempt < act["maximum_attempts"]:
                # Retry: no history event, just make the task available again
                # after the backoff delay.
                delay_ms = act["initial_interval_ms"] * (act["backoff_coefficient"] ** (attempt - 1))
                act["attempt"] = attempt + 1
                act["available_at"] = time.time() + delay_ms / 1000.0
                act["claimed"] = False
            else:
                del wf["pending_activities"][activity_id]
                self._append(wf, "ACTIVITY_TASK_FAILED", {
                    "activity_id": activity_id,
                    "error": params.get("error"),
                })
                wf["needs_wft"] = True
            self._persist()
            return {}

    def signal_workflow(self, params):
        with self.lock:
            wf = self._get(params["workflow_id"])
            if wf["status"] != "RUNNING":
                raise EngineError("WORKFLOW_CLOSED",
                                  f"workflow {wf['workflow_id']!r} is {wf['status']}")
            self._append(wf, "WORKFLOW_EXECUTION_SIGNALED", {
                "signal_name": params["signal_name"],
                "input": params.get("input"),
            })
            wf["needs_wft"] = True
            self._persist()
            return {}

    # ----- timer ticker -----

    def run_timer_loop(self):
        while True:
            time.sleep(0.05)
            with self.lock:
                now = time.time()
                fired = False
                for wf in self.workflows.values():
                    if wf["status"] != "RUNNING":
                        continue
                    for timer_id, fire_at in list(wf["timers"].items()):
                        if fire_at <= now:
                            del wf["timers"][timer_id]
                            self._append(wf, "TIMER_FIRED", {"timer_id": timer_id})
                            wf["needs_wft"] = True
                            fired = True
                if fired:
                    self._persist()


METHODS = frozenset({
    "ping", "start_workflow", "describe_workflow", "get_history",
    "poll_workflow_task", "complete_workflow_task",
    "poll_activity_task", "complete_activity_task", "fail_activity_task",
    "signal_workflow",
})


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        engine = self.server.engine
        for line in self.rfile:
            line = line.strip()
            if not line:
                continue
            request_id = None
            try:
                request = json.loads(line)
                request_id = request.get("id")
                method = request.get("method")
                if method not in METHODS:
                    raise EngineError("UNKNOWN_METHOD", f"unknown method {method!r}")
                result = getattr(engine, method)(request.get("params") or {})
                response = {"id": request_id, "result": result}
            except EngineError as e:
                response = {"id": request_id, "error": {"code": e.code, "message": e.message}}
            except Exception as e:  # malformed request
                response = {"id": request_id,
                            "error": {"code": "BAD_REQUEST", "message": str(e)}}
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

    engine = Engine(args.data_dir)
    threading.Thread(target=engine.run_timer_loop, daemon=True).start()

    server = Server(("127.0.0.1", args.port), Handler)
    server.engine = engine
    print(f"workflow engine listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
