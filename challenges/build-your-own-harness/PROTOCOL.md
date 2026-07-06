# Wire Protocol — Build your own harness

Build a **mini grader (harness)** — the same primitive behind open-crafters
itself: spawn user programs, proxy RPC calls to them, and assert outcomes with
`run_case`.

The real harness grades your harness **black-box** over TCP. Your harness, in
turn, spawns and tests a toy KV fixture shipped with this challenge.

## Process contract

```
./your_program.sh --port <port>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — may be passed; you may ignore it on your harness process.
  When you **spawn** child programs, always pass both `--port` and `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `spawn`

Start a subprocess and wait until it accepts TCP connections (up to **10s**).

- **params:** `{"program": "<path>"}` — executable or shell entry point
- **result:** `{"addr": "127.0.0.1:<port>"}`
- **errors:** `INVALID_PARAMS` — missing `program`; `SPAWN_FAILED` — child did
  not become ready in time

Spawn the child as:

```
<program> --port <free-port> --data-dir <temp-dir>
```

Pick a free loopback port and a fresh temp directory for each spawn. The child
must listen on `127.0.0.1`.

### `call`

Forward one NDJSON RPC to an already-running child address.

- **params:** `{"addr": "<host:port>", "method": "<string>", "params": {...}}`
- **result:** the child's `result` object (decoded JSON object)
- **errors:** transport failures; child's protocol `error` is propagated as an
  RPC error with the child's code and message

Open a TCP connection to `addr`, send one request line, read one response line,
and return the child's `result` or `error`.

### `run_case`

Spawn a program, call one method, and check the result.

- **params:**
  ```json
  {
    "program": "<path>",
    "method": "<string>",
    "params": {...},
    "expect": {<subset of result fields>}
  }
  ```
- **result:** `{}` on success
- **errors:** `CASE_FAILED` — result does not match `expect`; spawn/call errors
  as above

**Subset equality:** every key in `expect` must be present in the child's
`result` with an equal JSON value. Extra keys in the actual result are allowed.

Each `run_case` spawns a **fresh** child process.

## Toy KV fixture

Stage tests spawn the toy program at
`challenges/build-your-own-harness/fixtures/toy-kv/go/your_program.sh` (relative
to the open-crafters repo root when developing locally).

Toy methods:

| method | params | result |
|---|---|---|
| `ping` | `{}` | `{"message": "pong"}` |
| `set` | `{"key": "<string>", "value": "<string>"}` | `{}` |
| `get` | `{"key": "<string>"}` | `{"hit": true, "value": "<string>"}` or `{"hit": false}` |

## Error codes (harness)

| Code | When |
|---|---|
| `UNKNOWN_METHOD` | Unrecognized `method` on your harness |
| `INVALID_PARAMS` | Missing or invalid parameters |
| `SPAWN_FAILED` | Child did not listen within 10s |
| `CASE_FAILED` | `run_case` assertion mismatch |
