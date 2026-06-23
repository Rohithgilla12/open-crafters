"""Reference solution for the open-crafters "Build your own workflow SDK" challenge.

A deterministic workflow replay engine: given an event history, return the
commands workflow code would emit after replaying every event. Passes all 9
stages.
"""

import argparse
import json
import socketserver


KNOWN_TYPES = {"greet", "fetch", "timer_wait", "signal_wait", "pipeline"}


class EngineError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def validate_history(history):
    if not history:
        raise EngineError("INVALID_HISTORY", "history must not be empty")
    for i, event in enumerate(history):
        if event.get("event_id") != i + 1:
            raise EngineError(
                "INVALID_HISTORY",
                f"event_id must be sequential starting at 1, got {event.get('event_id')} at index {i}",
            )


def replay(workflow_type, history):
    if workflow_type not in KNOWN_TYPES:
        raise EngineError(
            "WORKFLOW_TYPE_NOT_FOUND",
            f'unknown workflow type "{workflow_type}"',
        )
    validate_history(history)

    last = history[-1]["type"]
    if last in ("WORKFLOW_EXECUTION_COMPLETED", "WORKFLOW_EXECUTION_FAILED"):
        return []

    if workflow_type == "greet":
        if last != "WORKFLOW_EXECUTION_STARTED":
            raise EngineError("INVALID_HISTORY", f"unexpected last event {last} for greet")
        inp = history[0]["attributes"].get("input") or {}
        name = inp.get("name", "")
        return [
            {
                "type": "COMPLETE_WORKFLOW",
                "attributes": {"result": {"greeting": f"hello {name}"}},
            }
        ]

    if workflow_type == "fetch":
        inp = history[0]["attributes"].get("input")
        if last == "WORKFLOW_EXECUTION_STARTED":
            return [
                {
                    "type": "SCHEDULE_ACTIVITY",
                    "attributes": {
                        "activity_id": "fetch",
                        "activity_type": "fetch",
                        "input": inp,
                    },
                }
            ]
        if last == "ACTIVITY_TASK_SCHEDULED":
            return []
        if last == "ACTIVITY_TASK_COMPLETED":
            result = history[-1]["attributes"].get("result")
            return [{"type": "COMPLETE_WORKFLOW", "attributes": {"result": result}}]
        raise EngineError("INVALID_HISTORY", f"unexpected last event {last} for fetch")

    if workflow_type == "timer_wait":
        if last == "WORKFLOW_EXECUTION_STARTED":
            return [
                {
                    "type": "START_TIMER",
                    "attributes": {"timer_id": "t1", "duration_ms": 500},
                }
            ]
        if last == "TIMER_STARTED":
            return []
        if last == "TIMER_FIRED":
            return [{"type": "COMPLETE_WORKFLOW", "attributes": {"result": "timer fired"}}]
        raise EngineError("INVALID_HISTORY", f"unexpected last event {last} for timer_wait")

    if workflow_type == "signal_wait":
        if last == "WORKFLOW_EXECUTION_STARTED":
            return []
        if last == "WORKFLOW_EXECUTION_SIGNALED":
            result = history[-1]["attributes"].get("input")
            return [{"type": "COMPLETE_WORKFLOW", "attributes": {"result": result}}]
        raise EngineError("INVALID_HISTORY", f"unexpected last event {last} for signal_wait")

    if workflow_type == "pipeline":
        if last == "WORKFLOW_EXECUTION_STARTED":
            return [
                {
                    "type": "SCHEDULE_ACTIVITY",
                    "attributes": {
                        "activity_id": "step1",
                        "activity_type": "work",
                        "input": None,
                    },
                }
            ]
        if last == "ACTIVITY_TASK_SCHEDULED":
            return []
        if last == "ACTIVITY_TASK_COMPLETED":
            return [
                {
                    "type": "START_TIMER",
                    "attributes": {"timer_id": "pause", "duration_ms": 100},
                }
            ]
        if last == "TIMER_STARTED":
            return []
        if last == "TIMER_FIRED":
            return [{"type": "COMPLETE_WORKFLOW", "attributes": {"result": "done"}}]
        raise EngineError("INVALID_HISTORY", f"unexpected last event {last} for pipeline")

    raise EngineError("WORKFLOW_TYPE_NOT_FOUND", f'unknown workflow type "{workflow_type}"')


def handle_request(method, params):
    if method == "ping":
        return {"message": "pong"}
    if method == "replay":
        workflow_type = params["workflow_type"]
        history = params.get("history") or []
        commands = replay(workflow_type, history)
        return {"commands": commands}
    raise EngineError("UNKNOWN_METHOD", f"unknown method {method!r}")


class RPCError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            request = json.loads(line)
            try:
                result = handle_request(request.get("method"), request.get("params") or {})
                response = {"id": request.get("id"), "result": result}
            except EngineError as e:
                response = {
                    "id": request.get("id"),
                    "error": {"code": e.code, "message": e.message},
                }
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
    print(f"listening on 127.0.0.1:{args.port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
