# Wire Protocol — Build your own job platform (gateway)

You implement the **orchestrator gateway** only. The harness spawns three
**reference** services (scheduler, queue, distributed-lock) and injects their
addresses into your environment before starting your program.

## Your process

```
./your_program.sh --port <port>
```

Environment variables set by the harness:

| Variable | Service |
|----------|---------|
| `SCHEDULER_ADDR` | Reference `build-your-own-scheduler` |
| `QUEUE_ADDR` | Reference `build-your-own-queue` |
| `LOCK_ADDR` | Reference `build-your-own-distributed-lock` |

Read each child's existing `PROTOCOL.md` in those challenges — speak NDJSON to
those addresses over TCP. Your gateway exposes a **new** protocol below.

## Architecture

Workers call your gateway, not the primitives directly:

1. **`submit_job`** → `schedule` on the reference scheduler
2. **Dispatcher** (under lock `dispatcher`) → `poll` due jobs from scheduler,
   `send` each to queue `jobs`
3. **`receive_work`** → run dispatcher, then `receive` from queue `jobs`
4. **`complete_work`** → `complete` on scheduler + `ack` on queue

Use queue name **`jobs`** and lock name **`dispatcher`** for singleton dispatch.

## Gateway transport

Newline-delimited JSON, same shape as other challenges.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `submit_job`

Schedule delayed work.

- **params:** `{"payload": <any>, "delay_ms": <int>}`
- **result:** `{"job_id": "<string>"}`

Forward to scheduler `schedule` with the same fields.

### `receive_work`

Fetch the next unit of work for a worker. **Non-blocking.**

- **params:** `{}`
- **result (work available):**
  ```json
  {
    "work": {
      "job_id": "j1",
      "payload": {"task": "email"},
      "lease_token": "<from scheduler poll>",
      "receipt": "<from queue receive>"
    }
  }
  ```
- **result (idle):** `{"work": null}`

Before receiving, try to move due scheduler jobs into queue `jobs` while
holding lock `dispatcher` (use `try_acquire` / `release`).

Queue message bodies should be JSON:
`{"job_id":"…","payload":…,"lease_token":"…"}`.

### `complete_work`

Mark a leased job successful and remove it from the delivery queue.

- **params:** `{"lease_token": "...", "receipt": "..."}`
- **result:** `{}`

Call scheduler `complete` then queue `ack`.

### `cancel_job`

Cancel a pending scheduled job.

- **params:** `{"job_id": "<string>"}`
- **result:** `{"cancelled": true}` or `{"cancelled": false}`

Proxy to scheduler `cancel`.

### `get_job`

Inspect job status (for debugging and tests).

- **params:** `{"job_id": "<string>"}`
- **result:** `{"status": "<pending|leased|completed|failed|cancelled>"}`

Proxy scheduler `get_job` status field.

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognized gateway method |
| `INVALID_PARAMS` | Missing required fields |

Child service errors may propagate as gateway `INTERNAL`.
