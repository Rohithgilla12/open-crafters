# Wire Protocol — Build your own distributed lock

Build a **distributed lock service**: clients acquire named locks with
time-bounded leases, renew or release them with a token, and query whether a
lock is held. Locks must be **exclusive** under contention, **expire**
automatically when leases run out, and stay **durable across crashes** so an
unexpired lock survives `SIGKILL` + restart.

The tester grades you entirely over TCP — and by `SIGKILL`ing your process to
verify that an active lock is still held after restart.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for durable state. From the **Durability** stage on,
  active locks must survive `SIGKILL` + restart with the same `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

## Model

The server holds a set of **locks**, each named by a string `name`. A lock is
either **free** or **held** by a `holder_id` until `expires_at_ms` (absolute
wall-clock epoch milliseconds). When `now >= expires_at_ms`, the lock is
treated as free — a new `acquire` may succeed.

Each successful acquisition issues a unique **`token`** string. `release` and
`renew` require the current token; a wrong or stale token is not an RPC error
for `release` (it returns `released: false`) but is `NOT_HOLDER` for `renew`.

All time is **wall-clock** epoch milliseconds. Lease expiry is computed from
absolute time, never from "time since boot" — that is what makes the
durability stage tractable.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `acquire`

Grant the lock if it is free or the current lease has expired. Otherwise error.

- **params:**
  - `name` (string, required) — lock name.
  - `holder_id` (string, required) — identity of the acquirer (opaque string).
  - `lease_ms` (int, required) — lease duration in milliseconds. Must be `≥ 1`.
- **result:**
  ```json
  {"token": "…", "expires_at_ms": 1710000000300}
  ```
  - `token` — unique per acquisition (UUID or random hex).
  - `expires_at_ms` — absolute time when the lease expires (`now + lease_ms` on
    a fresh grant).
- **errors:**
  - `LOCK_HELD` — another holder holds the lock with an unexpired lease.
  - `INVALID_PARAMS` — a required field is missing or `lease_ms < 1`.

### `try_acquire`

Same semantics as `acquire`, but never errors on contention.

- **params:** same as `acquire`.
- **result:**
  ```json
  {"acquired": true, "token": "…", "expires_at_ms": 1710000000300}
  ```
  or `{"acquired": false}` when the lock is held by another with an unexpired
  lease.
- **errors:** `INVALID_PARAMS` only (missing fields or `lease_ms < 1`).

### `release`

Release the lock if `token` matches the current holder.

- **params:**
  - `name` (string, required).
  - `token` (string, required).
- **result:** `{"released": true}` when the token matches the current holder;
  `{"released": false}` when the lock is free, expired, or the token does not
  match. Never an RPC error for a wrong token.

### `renew`

Extend the lease for the current holder.

- **params:**
  - `name` (string, required).
  - `token` (string, required).
  - `lease_ms` (int, required) — extension duration. Must be `≥ 1`.
- **result:** `{"expires_at_ms": …}` — new absolute expiry, computed as
  `max(now, current_expires_at_ms) + lease_ms`.
- **errors:**
  - `NOT_HOLDER` — lock is free/expired, or `token` does not match.
  - `INVALID_PARAMS` — `lease_ms < 1` or required fields missing.

### `status`

Report whether a lock is currently held (unexpired lease).

- **params:** `{"name": "…"}`
- **result when held:**
  ```json
  {"held": true, "holder_id": "…", "expires_at_ms": …, "token": "…"}
  ```
  The `token` field is included when `held=true` (helps tests verify state).
- **result when not held:** `{"held": false}` — omit optional fields or set
  them to null.

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognised `method`. |
| `LOCK_HELD` | `acquire` while another holder has an unexpired lease. |
| `NOT_HOLDER` | `renew` with wrong/missing token or on a free/expired lock. |
| `INVALID_PARAMS` | Missing required fields or `lease_ms < 1`. |

## Durability

From the **Durability** stage on:

- Active locks (name, holder, token, `expires_at_ms`) survive `SIGKILL` +
  restart.
- After restart, `expires_at_ms` is still interpreted as absolute wall-clock
  time: expired locks are free; unexpired locks remain held.
- Persist on each mutating call (`acquire`, `release`, `renew`). An atomic
  temp-file rename is enough — `SIGKILL` is the threat model, not power loss.
