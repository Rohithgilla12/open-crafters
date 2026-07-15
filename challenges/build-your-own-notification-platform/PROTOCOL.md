# Wire Protocol — Build your own notification platform (gateway)

You implement the **notification gateway** only. The harness spawns three
**reference** services (queue, scheduler, rate-limiter) and injects their
addresses into your environment before starting your program.

## Your process

```
./your_program.sh --port <port>
```

Environment variables set by the harness:

| Variable | Service |
|----------|---------|
| `QUEUE_ADDR` | Reference `build-your-own-queue` |
| `SCHEDULER_ADDR` | Reference `build-your-own-scheduler` |
| `RATE_LIMITER_ADDR` | Reference `build-your-own-rate-limiter` |

Read each child's existing `PROTOCOL.md` in those challenges — speak NDJSON to
those addresses over TCP. Your gateway exposes a **new** protocol below.

## Architecture

Clients call your gateway, not the primitives directly:

1. **`configure_limit`** → `configure` on the rate-limiter (per-user key)
2. **`notify`** → `take` on the rate-limiter; if allowed, `send` to queue
   `notifications`
3. **`schedule_notify`** → `schedule` on the scheduler (payload is the
   notification envelope)
4. **Dispatcher** (inside `receive`) → `poll` due jobs from the scheduler,
   `send` each envelope to queue `notifications`, then `complete` the leased
   scheduler job
5. **`receive`** → run dispatcher, then `receive` from queue `notifications`
6. **`ack`** → `ack` on the queue

Use queue name **`notifications`**. Rate-limiter keys are **`user:<user_id>`**.

Notification envelope JSON (queued and scheduled):

```json
{
  "notification_id": "<string>",
  "user_id": "<string>",
  "channel": "<string>",
  "body": "<string>"
}
```

`notification_id` is assigned by the gateway (any unique string).

## Gateway transport

Newline-delimited JSON, same shape as other challenges.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `configure_limit`

Create or replace the per-user rate limiter.

- **params:** `{"user_id": "<string>", "limit": <int>, "window_ms": <int>}`
- **result:** `{}`

Forward to rate-limiter `configure` with:
`key="user:<user_id>"`, `algorithm="fixed_window"`, and the given `limit` /
`window_ms`.

### `notify`

Send an immediate notification if the user's limiter allows it.

- **params:** `{"user_id": "<string>", "channel": "<string>", "body": "<string>"}`
- **result (allowed):** `{"notification_id": "<string>", "queued": true}`
- **result (denied):** `{"notification_id": null, "queued": false, "rate_limited": true}`

Call rate-limiter `take` with `key="user:<user_id>"` and `cost=1`. When
`allowed`, enqueue the envelope on queue `notifications` and return
`queued=true`. When denied, do **not** enqueue; return `rate_limited=true`.

### `schedule_notify`

Schedule a delayed notification (digests / quiet hours).

- **params:** `{"user_id": "<string>", "channel": "<string>", "body": "<string>", "delay_ms": <int>}`
- **result:** `{"notification_id": "<string>", "job_id": "<string>"}`

Mint a `notification_id`, then call scheduler `schedule` with
`delay_ms` and `payload` equal to the envelope JSON object (not a string).

### `receive`

Fetch the next delivery for a worker. **Non-blocking.**

- **params:** `{}`
- **result (message available):**
  ```json
  {
    "notification": {
      "notification_id": "...",
      "user_id": "...",
      "channel": "...",
      "body": "...",
      "receipt": "<from queue receive>"
    }
  }
  ```
- **result (idle):** `{"notification": null}`

Before receiving, poll the scheduler and move due jobs into queue
`notifications` (decode the scheduled payload as the envelope and `send` its
JSON string form).

### `ack`

Remove a delivered notification from the queue.

- **params:** `{"receipt": "<string>"}`
- **result:** `{}`

Proxy to queue `ack`.

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognized gateway method |
| `INVALID_PARAMS` | Missing required fields |

Child service errors may propagate as gateway `INTERNAL`. Rate-limiter
`KEY_NOT_FOUND` on `notify` should surface as gateway `INVALID_PARAMS` (limit
not configured).
