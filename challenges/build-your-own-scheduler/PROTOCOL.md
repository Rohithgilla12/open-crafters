# Wire Protocol — Build your own scheduler

Build a **durable job scheduler**: workers poll for due work, jobs run after a
delay (or at an absolute time), leases prevent double-delivery, failed jobs
retry, and recurring jobs reschedule themselves.

The tester grades you entirely over TCP — and by `SIGKILL`ing your process to
verify that scheduled fire times survive restarts.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for durable state. From the **Durability** stage on,
  acknowledged schedules must survive `SIGKILL` + restart with the same
  `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

## Job lifecycle

| status | meaning |
|---|---|
| `pending` | waiting until `run_at_ms`, or leased but lease expired |
| `leased` | handed to a worker by `poll`; hidden until lease expires |
| `completed` | worker called `complete` |
| `failed` | retries exhausted |
| `cancelled` | cancelled while still pending |

Workers **poll** for due jobs (non-blocking). A polled job is **leased** for
`lease_ms` (default **3000**). If the worker does not `complete` or `fail`
before the lease expires, the job becomes pollable again with a **new**
`lease_token` and the same `attempt`.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `schedule`

Enqueue a job.

- **params:**
  - `payload` (any JSON value) — opaque job data delivered to workers.
  - **Exactly one of:**
    - `delay_ms` (int) — run at `now + delay_ms` (wall clock).
    - `run_at_ms` (int) — absolute epoch milliseconds.
  - `lease_ms` (int, optional) — lease duration after `poll`; default **3000**.
  - `retry_policy` (optional):
    ```json
    {"maximum_attempts": 3, "retry_delay_ms": 200}
    ```
    Default: `maximum_attempts: 1` (no retries).
  - `interval_ms` (int, optional) — after a **successful** `complete`, enqueue
    another job with the same `payload` at `now + interval_ms`.
- **result:** `{"job_id": "<server-generated unique string>"}`
- **durability (from Durability stage):** only return `job_id` after the job is
  durably stored. `run_at_ms` must be persisted as an **absolute** time — after
  restart, fire when wall clock reaches `run_at_ms`, not "remaining delay from
  boot".

### `poll`

Fetch the next due job. **Non-blocking.**

- **params:** `{}`
- **result:** when a job is available:
  ```json
  {
    "job": {
      "job_id": "j1",
      "payload": {"task": "email"},
      "attempt": 1,
      "lease_token": "<opaque>"
    }
  }
  ```
  or `{"job": null}` when nothing is due.
- Returns the due job with the **earliest** `run_at_ms`. Ties: any order is fine.
- At most one worker receives a given job at a time (lease).

### `complete`

Mark a leased job successful.

- **params:** `{"lease_token": "...", "result": <any optional>}`
- **result:** `{}`
- **errors:** `LEASE_NOT_FOUND` — unknown or expired token.
- **effects:** status → `completed`. If `interval_ms` was set on schedule,
  enqueue the next run.

### `fail`

Report job failure; may retry per `retry_policy`.

- **params:** `{"lease_token": "...", "error": "<string>"}`
- **result:** `{}`
- **errors:** `LEASE_NOT_FOUND`
- **effects:** if `attempt < maximum_attempts`, reschedule at
  `now + retry_delay_ms` with `attempt + 1`. Otherwise status → `failed`.

### `cancel`

Cancel a pending job (not yet completed/failed/cancelled).

- **params:** `{"job_id": "..."}`
- **result:** `{"cancelled": true}` if the job was pending and is now cancelled,
  else `{"cancelled": false}`.
- **errors:** `JOB_NOT_FOUND`

### `get_job`

- **params:** `{"job_id": "..."}`
- **result:**
  ```json
  {
    "job_id": "j1",
    "status": "pending",
    "payload": {"task": "email"},
    "run_at_ms": 1710000000000,
    "attempt": 1,
    "result": null,
    "error": null
  }
  ```
- **errors:** `JOB_NOT_FOUND`
