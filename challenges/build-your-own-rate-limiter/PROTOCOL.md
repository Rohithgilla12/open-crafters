# Wire Protocol — Build your own rate limiter

Build a **rate-limiting service**: clients configure named limiters and then
ask, request by request, whether an action is allowed. You implement three
classic algorithms — **fixed window**, **token bucket**, and **sliding
window** — admit traffic atomically under concurrency, and keep limiter state
**durable across crashes** so a restart can't hand out a free burst.

The tester grades you entirely over TCP — and by `SIGKILL`ing your process to
verify that a drained limiter stays drained after restart.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for durable state. From the **Durability** stage on,
  configured limiters and their consumption must survive `SIGKILL` + restart
  with the same `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

## Model

The server holds a set of **limiters**, each named by a string `key`. A
limiter is created (or replaced) with `configure` and consumed with `take`;
`peek` reports its state without consuming. There is no global limiter —
every `take`/`peek` names a `key`, and `take`/`peek` on a key that was never
configured is an error.

All time is **wall-clock** epoch milliseconds. Refill and window boundaries
are computed from absolute time, never from "time since boot" — that is what
makes the durability stage tractable.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `configure`

Create or **replace** the limiter for a key. Reconfiguring an existing key
resets its consumption (a token bucket starts **full**; window counters start
empty).

- **params:**
  - `key` (string, required).
  - `algorithm` (string, required): `"token_bucket"`, `"fixed_window"`, or
    `"sliding_window"`.
  - **token_bucket:**
    - `capacity` (int) — maximum tokens; the bucket starts full.
    - `refill_tokens` (int) — tokens added every `refill_interval_ms`.
    - `refill_interval_ms` (int) — refill period. Refill is **continuous**:
      after `t` ms, `t / refill_interval_ms * refill_tokens` tokens have
      accrued (fractional tokens allowed internally), capped at `capacity`.
  - **fixed_window / sliding_window:**
    - `limit` (int) — maximum admitted cost per window.
    - `window_ms` (int) — window length.
- **result:** `{}`
- **errors:**
  - `INVALID_ALGORITHM` — `algorithm` is not one of the three above.
  - `INVALID_PARAMS` — a required parameter is missing.

### `take`

Attempt to admit a request against a key's limiter.

- **params:**
  - `key` (string, required).
  - `cost` (int, optional, default **1**) — units this request consumes. Must
    be a positive integer. Behaviour for `cost` greater than the limiter's
    `capacity`/`limit` is unspecified.
- **result:**
  ```json
  {"allowed": true, "remaining": 4, "limit": 5, "retry_after_ms": 0}
  ```
  - `allowed` — whether the request is permitted. When `true`, `cost` units are
    consumed; when `false`, **nothing** is consumed.
  - `limit` — the limiter's `capacity` (token bucket) or `limit` (windows).
  - `remaining` — units left **after** this call. Token bucket: the available
    tokens rounded **down** to an integer. Windows: `limit` minus the cost
    admitted in the current window.
  - `retry_after_ms` — `0` when `allowed`. When denied, a **lower bound** on
    the milliseconds until a retry of the **same `cost`** could succeed:
    - token bucket: time for the deficit `cost − tokens` to refill.
    - fixed window: time until the current window ends.
    - sliding window: time until enough of the oldest in-window cost ages out.
- **errors:** `KEY_NOT_FOUND` — the key was never configured.

### `peek`

Report a key's current state **without consuming**. Two consecutive `peek`s
with no intervening `take` must return the same `remaining` (modulo refill).

- **params:** `{"key": "...", "cost": <int, optional, default 1>}`
- **result:**
  ```json
  {"remaining": 2, "limit": 5, "retry_after_ms": 0}
  ```
  - `remaining` — units available right now (token bucket: floored tokens;
    windows: `limit` minus current in-window cost).
  - `limit` — as in `take`.
  - `retry_after_ms` — `0` if a `take` of `cost` would be admitted right now,
    otherwise a lower bound on the wait until it would be.
- **errors:** `KEY_NOT_FOUND`.

## Algorithm semantics

The three algorithms differ only in how `take`/`peek` decide admission.

### token_bucket

The bucket holds up to `capacity` tokens and refills continuously at
`refill_tokens / refill_interval_ms` tokens per ms. A `take` of `cost` is
admitted iff the available tokens are `≥ cost`; on admission, `cost` tokens
are removed. Tokens are tracked with fractional precision internally but
reported (`remaining`) as the floor.

### fixed_window

Time is divided into windows of `window_ms` aligned to the epoch: the window
for time `now` is `floor(now / window_ms)`. Each window admits up to `limit`
units of cost; the count resets to zero the instant the window index changes.
A `take` of `cost` is admitted iff `count + cost ≤ limit`.

Fixed windows are simple but allow a **boundary burst**: up to `2 × limit`
admissions across the seam of two adjacent windows. The next algorithm fixes
exactly that.

### sliding_window

A `take` of `cost` is admitted iff the total cost admitted in the trailing
`window_ms` (the half-open interval `(now − window_ms, now]`) plus `cost` is
`≤ limit`. Equivalently, keep the timestamps of admitted units and count only
those newer than `now − window_ms`. This removes the boundary burst: no
`window_ms`-wide span ever admits more than `limit`.

## Durability

From the **Durability** stage on:

- Configured limiters (algorithm + parameters) survive `SIGKILL` + restart.
- **Consumption** survives too. A bucket drained to empty just before a crash
  is still empty just after restart — a restart must not refill it for free.
  Because refill and windows are computed from absolute wall-clock time,
  tokens that *legitimately* accrued during the downtime are still available;
  what must not happen is the limiter resetting to full.
