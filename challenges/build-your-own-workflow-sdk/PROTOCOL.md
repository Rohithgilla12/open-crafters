# Wire Protocol — Build your own workflow SDK

This document specifies the wire protocol your **deterministic workflow replay
engine** must implement. The open-crafters tester sends **event histories** and
asserts on the **commands** your engine emits when replaying them — the same
events-in/commands-out loop workers perform against a Temporal server, but
without a server in the loop.

Your program is a **replay runtime**. The tester never reads your code — if you
speak this protocol correctly, you pass, in any language.

**Prerequisite:** [Build your own Temporal](../build-your-own-temporal/) — you
should already understand event histories and commands. Event and command shapes
match that challenge's [PROTOCOL.md](../build-your-own-temporal/PROTOCOL.md).

## Process contract

Your program is started as:

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — a directory you may use if you wish. The tester does not
  require persistence for this challenge; replay is evaluated statelessly per
  request.

Your server must be accepting connections within **10 seconds** of being
started, and must handle **multiple concurrent connections**.

## Transport: newline-delimited JSON

Messages are JSON objects, one per line (`\n`-terminated), over a plain TCP
connection. A client sends a *request* and reads one *response* line for it.
Requests on a single connection are sent sequentially.

**Request:**

```json
{"id": "42", "method": "replay", "params": {"workflow_type": "greet", "history": [...]}}
```

**Successful response** (echo the request `id`):

```json
{"id": "42", "result": {"commands": [{"type": "COMPLETE_WORKFLOW", "attributes": {"result": {"greeting": "hello world"}}}]}}
```

**Error response:**

```json
{"id": "42", "error": {"code": "WORKFLOW_TYPE_NOT_FOUND", "message": "unknown workflow type \"nope\""}}
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

### `replay`

Replay the given event history for a workflow type and return the **commands**
the workflow code would emit after processing every event in order.

- **params:**
  - `workflow_type` (string) — which workflow definition to run.
  - `history` (array of Event) — the complete history so far; see [Events](#events).
- **result:** `{"commands": [Command, ...]}` — see [Commands](#commands). The
  list may be **empty** when the workflow is waiting for more history events
  (nothing to do yet).
- **errors:**
  - `WORKFLOW_TYPE_NOT_FOUND` — no definition registered for this type.
  - `INVALID_HISTORY` — history is malformed (non-sequential `event_id`, unknown
    event type, impossible ordering, etc.).

#### Replay semantics

1. **Deterministic:** calling `replay` twice with the same `workflow_type` and
   `history` **must** return identical `commands` (same types and attributes).
   Your engine must not use wall-clock time, randomness, or external I/O when
   deciding commands.
2. **Events are facts, commands are decisions:** events already in `history`
   happened in a previous workflow task. During replay you must **not** re-emit
   commands for past events — only return commands the workflow would produce
   **now**, after replaying the full history.
3. **Terminal histories:** if the last event is `WORKFLOW_EXECUTION_COMPLETED`
   or `WORKFLOW_EXECUTION_FAILED`, return `{"commands": []}` — the workflow is
   finished.
4. **Waiting:** if the workflow has nothing to do until a new event arrives
   (e.g. it scheduled an activity and is waiting for completion), return
   `{"commands": []}`.

## Events

An event is:

```json
{"event_id": 1, "type": "WORKFLOW_EXECUTION_STARTED", "attributes": {...}}
```

`event_id` is a positive integer, starting at **1**, incrementing by 1 with no
gaps, scoped to a single replay invocation.

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

| type | attributes |
|---|---|
| `SCHEDULE_ACTIVITY` | `activity_id`, `activity_type`, `input` |
| `START_TIMER` | `timer_id`, `duration_ms` |
| `COMPLETE_WORKFLOW` | `result` |
| `FAIL_WORKFLOW` | `error` |

## Built-in workflow types

The tester exercises these workflow definitions. Your engine must implement
exactly the behavior described below.

### `greet`

- **Input** (in `WORKFLOW_EXECUTION_STARTED.input`): `{"name": "<string>"}`
- After replaying a history whose last event is `WORKFLOW_EXECUTION_STARTED`:
  emit one `COMPLETE_WORKFLOW` with
  `{"result": {"greeting": "hello <name>"}}`.

### `fetch`

- **Input:** any JSON value (passed through to the activity).
- After `WORKFLOW_EXECUTION_STARTED` only: emit `SCHEDULE_ACTIVITY` with
  `activity_id: "fetch"`, `activity_type: "fetch"`, `input` copied from
  workflow input.
- After `ACTIVITY_TASK_COMPLETED` for `activity_id: "fetch"`: emit
  `COMPLETE_WORKFLOW` with `result` set to the activity's `result` attribute.
- While waiting for the activity (history ends at `ACTIVITY_TASK_SCHEDULED`):
  emit no commands.

### `timer_wait`

- After `WORKFLOW_EXECUTION_STARTED` only: emit `START_TIMER` with
  `timer_id: "t1"`, `duration_ms: 500`.
- After `TIMER_FIRED` for `timer_id: "t1"`: emit `COMPLETE_WORKFLOW` with
  `result: "timer fired"`.
- While waiting for the timer (history ends at `TIMER_STARTED`): emit no
  commands.

### `signal_wait`

- After `WORKFLOW_EXECUTION_STARTED` only: emit no commands (waiting for a
  signal).
- After `WORKFLOW_EXECUTION_SIGNALED` with `signal_name: "go"`: emit
  `COMPLETE_WORKFLOW` with `result` set to the signal's `input`.

### `pipeline`

Orchestrates activity → timer → complete (used in the gauntlet).

- After `WORKFLOW_EXECUTION_STARTED` only: emit `SCHEDULE_ACTIVITY` with
  `activity_id: "step1"`, `activity_type: "work"`, `input: null`.
- After `ACTIVITY_TASK_COMPLETED` for `step1`: emit `START_TIMER` with
  `timer_id: "pause"`, `duration_ms: 100`.
- After `TIMER_FIRED` for `pause`: emit `COMPLETE_WORKFLOW` with
  `result: "done"`.
- While waiting at any intermediate step: emit no commands.
