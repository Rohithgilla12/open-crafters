# Wire Protocol — Build your own ID generator

Build a **snowflake-style ID generator**: monotonically sortable 64-bit IDs
composed from a timestamp, a configurable worker id, and a per-millisecond
sequence. IDs must stay **unique under concurrency**, survive **crash +
restart** without reuse, and support **batch allocation**.

The tester grades you entirely over TCP — and by `SIGKILL`ing your process to
verify that ID generation never moves backwards after restart.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for durable state. From the **Durability** stage on,
  the last issued `(timestamp_ms, sequence)` must survive `SIGKILL` + restart
  with the same `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

## ID layout

IDs are **positive decimal strings** (JSON cannot safely hold all 64-bit values
as numbers). Internally they are 64-bit integers with this bit layout:

| Field | Bits | Notes |
|-------|------|-------|
| (unused) | 1 | Always 0 — IDs are positive |
| `timestamp_ms` | 41 | Milliseconds since **epoch** `1577836800000` (2020-01-01 UTC) |
| `worker_id` | 10 | `0`–`1023`, set via `configure` |
| `sequence` | 12 | `0`–`4095`, increments within the same millisecond |

Composition:

```
id = ((timestamp_ms - 1577836800000) << 22) | (worker_id << 12) | sequence
```

`parse` decodes a decimal `id` back to `{timestamp_ms, worker_id, sequence}`
using absolute wall-clock `timestamp_ms` (not the shifted value).

When the sequence overflows within the same millisecond (`4095` → next id in
that ms), wait until the system clock advances to the **next millisecond**
before issuing more IDs. If the wall clock moves **backwards** below the last
issued timestamp, return `CLOCK_BACKWARDS`.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `configure`

Set the worker id used in all subsequent IDs.

- **params:** `{"worker_id": <int>}` — required, `0`–`1023`.
- **result:** `{}`
- **errors:** `INVALID_PARAMS` — missing or out-of-range `worker_id`.

Default worker id is `0` until configured.

### `next_id`

Allocate one new unique ID.

- **params:** `{}` (no fields required; extra fields → `INVALID_PARAMS`).
- **result:** `{"id": "<decimal string>"}`
- **errors:** `CLOCK_BACKWARDS` — wall clock moved before last issued timestamp.

### `batch`

Allocate many IDs in one call. IDs must be **strictly increasing** in the
returned array.

- **params:** `{"count": <int>}` — required, `1`–`1024`.
- **result:** `{"ids": ["...", "..."]}`
- **errors:**
  - `INVALID_PARAMS` — missing `count` or `count < 1`.
  - `BATCH_TOO_LARGE` — `count > 1024`.
  - `CLOCK_BACKWARDS` — same as `next_id`.

### `parse`

Debug helper — decode an ID (tests use this to verify your bit layout).

- **params:** `{"id": "<decimal string>"}` — required.
- **result:**
  ```json
  {"timestamp_ms": 1710000000123, "worker_id": 42, "sequence": 7}
  ```
- **errors:** `INVALID_PARAMS` — missing or non-numeric `id`.

## Durability

Persist enough state in `--data-dir` that after `SIGKILL` + restart:

1. The next issued ID is **strictly greater** than every ID issued before the
   crash.
2. No ID is **reused**.

Typically: persist `last_timestamp_ms`, `last_sequence`, and `worker_id` after
each allocation (atomic rename is fine).

## Errors (summary)

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unsupported `method` |
| `INVALID_PARAMS` | Bad or missing parameters |
| `BATCH_TOO_LARGE` | `batch.count > 1024` |
| `CLOCK_BACKWARDS` | `now < last_timestamp_ms` |
