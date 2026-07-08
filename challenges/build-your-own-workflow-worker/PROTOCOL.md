# Wire Protocol — Build your own workflow worker (gateway)

You implement the **worker gateway** only. The harness spawns two **reference**
services (Temporal server and workflow SDK) and injects their addresses into
your environment before starting your program.

## Your process

```
./your_program.sh --port <port>
```

| Variable | Service |
|----------|---------|
| `TEMPORAL_ADDR` | Reference `build-your-own-temporal` |
| `SDK_ADDR` | Reference `build-your-own-workflow-sdk` |

Speak NDJSON to those addresses over TCP (see each challenge's `PROTOCOL.md`).
Your gateway exposes a **new** protocol below.

## Architecture

Workers (and the tester) call your gateway instead of Temporal + SDK directly:

1. **`run_workflow`** — `start_workflow` on Temporal, then drive the worker loop
   until the execution reaches `COMPLETED` or `FAILED`.
2. **Worker loop** — poll workflow tasks → `replay` on the SDK →
   `complete_workflow_task`; poll activity tasks → stub results →
   `complete_activity_task`.
3. **`start_workflow` / `await_workflow` / `signal_workflow`** — split API for
   workflows that block on external signals.

Use task queue **`default`** unless a method accepts `task_queue`.

### Activity stubs

When you complete activity tasks, return deterministic stubs:

| `activity_type` | `result` |
|-----------------|----------|
| `fetch` | `{"status": 200, "body": "ok"}` |
| `work` | `{"done": true}` |
| (other) | `{"ok": true}` |

## Gateway transport

Newline-delimited JSON, same shape as other challenges.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `run_workflow`

Start a workflow and block until it finishes.

- **params:** `{"workflow_id": "...", "workflow_type": "...", "input": <any>, "task_queue": "default"}`
- **result:** `{"status": "COMPLETED"|"FAILED"|"RUNNING", "result": <any>, "error": <any>}`
- **errors:** forward Temporal codes such as `WORKFLOW_ALREADY_EXISTS`

### `start_workflow`

Start without waiting (for signal-driven flows).

- **params:** same as Temporal `start_workflow`
- **result:** `{"run_id": "..."}`

### `signal_workflow`

Forward to Temporal `signal_workflow`.

### `await_workflow`

Drive the worker loop until the workflow reaches a terminal state.

- **params:** `{"workflow_id": "..."}`
- **result:** same shape as `run_workflow`
