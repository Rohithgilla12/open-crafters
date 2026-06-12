# Wire Protocol — Build your own Temporal

This document specifies the wire protocol your workflow engine server must
implement. The open-crafters tester is the only client you need to satisfy:
it plays the role of *frontend clients* (starting workflows, querying state)
and *workers* (polling for tasks, executing them, reporting results).

Your program is a **server**. The tester never reads your code — if you speak
this protocol correctly, you pass, in any language.

## Process contract

Your program is started as:

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — a directory you may use for persistence. It exists and is
  writable. Early stages can ignore it; from the **Durability** stage onward,
  your server must survive a `SIGKILL` + restart with the same `--data-dir`
  without losing workflow state.

Your server must be accepting connections within **10 seconds** of being
started, and must handle **multiple concurrent connections**.

## Transport: newline-delimited JSON

Messages are JSON objects, one per line (`\n`-terminated), over a plain TCP
connection. A client sends a *request* and reads one *response* line for it.
Requests on a single connection are sent sequentially.

**Request:**

```json
{"id": "42", "method": "start_workflow", "params": {"workflow_id": "wf-1", "workflow_type": "greet", "input": {"name": "world"}, "task_queue": "default"}}
```

**Successful response** (echo the request `id`):

```json
{"id": "42", "result": {"run_id": "8f1d2c3e"}}
```

**Error response:**

```json
{"id": "42", "error": {"code": "WORKFLOW_ALREADY_EXISTS", "message": "workflow wf-1 is already running"}}
```

- `id` is an opaque string chosen by the client; echo it back verbatim.
- A response must contain exactly one of `result` or `error`.
- `error.code` values are specified per method below. `error.message` is
  free-form human-readable text.
- Unknown methods should return error code `UNKNOWN_METHOD`.

## Methods

### `ping`

Liveness check.

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `start_workflow`

Start a new workflow execution.

- **params:**
  - `workflow_id` (string) — caller-chosen identifier, unique among
    currently-known workflows.
  - `workflow_type` (string) — name of the workflow definition; opaque to the
    server, delivered to workers.
  - `input` (any JSON value) — workflow input, opaque to the server.
  - `task_queue` (string) — queue on which workflow tasks for this execution
    are dispatched.
- **result:** `{"run_id": "<server-generated unique string>"}`
- **errors:** `WORKFLOW_ALREADY_EXISTS` — a workflow with this `workflow_id`
  already exists.
- **effects:** appends event `WORKFLOW_EXECUTION_STARTED` to a fresh history
  and schedules a workflow task on `task_queue`.

### `describe_workflow`

- **params:** `{"workflow_id": "..."}`
- **result:**
  ```json
  {
    "workflow_id": "wf-1",
    "run_id": "8f1d2c3e",
    "workflow_type": "greet",
    "status": "RUNNING",
    "result": null,
    "error": null
  }
  ```
  - `status` is one of `RUNNING`, `COMPLETED`, `FAILED`.
  - `result` is set (any JSON value) when status is `COMPLETED`.
  - `error` is set (string) when status is `FAILED`.
- **errors:** `WORKFLOW_NOT_FOUND`

### `get_history`

- **params:** `{"workflow_id": "..."}`
- **result:** `{"events": [Event, ...]}` — see [Events](#events).
- **errors:** `WORKFLOW_NOT_FOUND`

### `poll_workflow_task`

Workers call this to receive workflow tasks. **Non-blocking**: if no task is
available, return immediately with `{"task": null}`; clients poll in a loop.

- **params:** `{"task_queue": "default"}`
- **result** when a task is available:
  ```json
  {
    "task": {
      "task_token": "<opaque unique string>",
      "workflow_id": "wf-1",
      "run_id": "8f1d2c3e",
      "workflow_type": "greet",
      "history": [Event, ...]
    }
  }
  ```
  `history` is the **complete** event history so far, in order.
- A delivered task is *claimed*: it must not be delivered again (except after
  a server restart, when unfinished claims are forgotten and the task is
  re-delivered).
- At most one workflow task per workflow execution may be outstanding at any
  time. If new events occur while one is outstanding, deliver a fresh task
  (with the fuller history) only after the outstanding one is completed.

### `complete_workflow_task`

The worker reports the decisions ("commands") the workflow code produced.

- **params:** `{"task_token": "...", "commands": [Command, ...]}` — see
  [Commands](#commands). The list may be empty (the workflow is just waiting
  for more events).
- **result:** `{}`
- **errors:** `TASK_NOT_FOUND` — unknown or already-completed token.
- **effects:** apply each command in order (append events, schedule activity
  tasks, start timers, finish the workflow).

### `poll_activity_task`

Same non-blocking semantics as `poll_workflow_task`.

- **params:** `{"task_queue": "default"}`
- **result** when a task is available:
  ```json
  {
    "task": {
      "task_token": "<opaque unique string>",
      "workflow_id": "wf-1",
      "run_id": "8f1d2c3e",
      "activity_id": "a1",
      "activity_type": "send_email",
      "input": {"to": "x@y.z"},
      "attempt": 1
    }
  }
  ```
  `attempt` starts at 1 and increments on each retry.

### `complete_activity_task`

- **params:** `{"task_token": "...", "result": <any JSON value>}`
- **result:** `{}`
- **errors:** `TASK_NOT_FOUND`
- **effects:** appends `ACTIVITY_TASK_COMPLETED` and schedules a workflow
  task.

### `fail_activity_task`

- **params:** `{"task_token": "...", "error": "<string>"}`
- **result:** `{}`
- **errors:** `TASK_NOT_FOUND`
- **effects:** if the activity's retry policy allows another attempt, schedule
  a retry (see [Retry policies](#retry-policies)) — **no history event** is
  recorded for a retried failure. If attempts are exhausted, append
  `ACTIVITY_TASK_FAILED` and schedule a workflow task.

### `signal_workflow`

- **params:** `{"workflow_id": "...", "signal_name": "...", "input": <any>}`
- **result:** `{}`
- **errors:** `WORKFLOW_NOT_FOUND`; `WORKFLOW_CLOSED` if the workflow is not
  `RUNNING`.
- **effects:** appends `WORKFLOW_EXECUTION_SIGNALED` and schedules a workflow
  task.

## Events

An event is:

```json
{"event_id": 1, "type": "WORKFLOW_EXECUTION_STARTED", "attributes": {...}}
```

`event_id` is a positive integer, starting at **1**, incrementing by 1 per
event, scoped to a single workflow execution. History is append-only.

| type | attributes |
|---|---|
| `WORKFLOW_EXECUTION_STARTED` | `workflow_type`, `input` |
| `ACTIVITY_TASK_SCHEDULED` | `activity_id`, `activity_type`, `input` |
| `ACTIVITY_TASK_COMPLETED` | `activity_id`, `result` |
| `ACTIVITY_TASK_FAILED` | `activity_id`, `error` |
| `TIMER_STARTED` | `timer_id`, `duration_ms` |
| `TIMER_FIRED` | `timer_id` |
| `WORKFLOW_EXECUTION_SIGNALED` | `signal_name`, `input` |
| `WORKFLOW_EXECUTION_COMPLETED` | `result` |
| `WORKFLOW_EXECUTION_FAILED` | `error` |

## Commands

A command is:

```json
{"type": "SCHEDULE_ACTIVITY", "attributes": {...}}
```

| type | attributes | effect |
|---|---|---|
| `SCHEDULE_ACTIVITY` | `activity_id`, `activity_type`, `input`, optional `retry_policy` | append `ACTIVITY_TASK_SCHEDULED`; make an activity task available on the workflow's task queue |
| `START_TIMER` | `timer_id`, `duration_ms` | append `TIMER_STARTED`; after `duration_ms` elapses, append `TIMER_FIRED` and schedule a workflow task |
| `COMPLETE_WORKFLOW` | `result` | append `WORKFLOW_EXECUTION_COMPLETED`; status → `COMPLETED` |
| `FAIL_WORKFLOW` | `error` | append `WORKFLOW_EXECUTION_FAILED`; status → `FAILED` |

## Retry policies

```json
{"maximum_attempts": 3, "initial_interval_ms": 200, "backoff_coefficient": 2.0}
```

- Default when `retry_policy` is omitted: `maximum_attempts: 1` (no retries).
- After a failure of attempt `n` (1-based), if `n < maximum_attempts`, the
  activity task becomes available again after a delay of
  `initial_interval_ms × backoff_coefficient^(n-1)` milliseconds, with
  `attempt = n + 1`.
- Only the *final* failure (attempts exhausted) is recorded in history.

## Workflow task scheduling — summary

A workflow task must be made available on the workflow's task queue whenever
the workflow needs to make progress, i.e. after any of:

- `WORKFLOW_EXECUTION_STARTED`
- `ACTIVITY_TASK_COMPLETED` / `ACTIVITY_TASK_FAILED`
- `TIMER_FIRED`
- `WORKFLOW_EXECUTION_SIGNALED`

subject to the *at most one outstanding workflow task per execution* rule.
Multiple trigger events may be coalesced into a single workflow task.

## Durability contract

From the Durability stage onward, the tester will `SIGKILL` your process at
arbitrary points and restart it with the same `--data-dir`. After restart:

- All workflows, their histories, statuses, and results must be intact.
- Pending timers must still fire (based on wall-clock time, not "remaining
  time from boot").
- Activity tasks and workflow tasks that were *available* or *claimed but not
  completed* at the time of the kill must become available again (old task
  tokens may be invalidated).
