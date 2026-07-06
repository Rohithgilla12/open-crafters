# Walkthrough — Build your own harness

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint harness` prints just the hint for your next stage;
`crafters walkthrough harness --stage <slug>` prints one section.

## bind — Boot the harness

> **Hint:** Same newline-delimited JSON loop as every challenge: read line,
> decode, dispatch, respond, flush. `ping` returns `pong` — wire transport once,
> then add harness methods stage by stage.

**How it works:** The reference isolates RPC dispatch from TCP handling. Each
connection is independent. Your harness process ignores its own `--data-dir`.

## spawn — Spawn a program

> **Hint:** Reserve a free loopback port, create a temp `--data-dir`, start the
> child with both flags, poll TCP for up to 10s, return `{"addr": "127.0.0.1:port"}`.
> Track child PIDs so you can reap them later.

**How it works:** Port reservation prevents races. The tester spawns the toy KV
fixture and pings the returned address directly.

## call — Proxy a call

> **Hint:** Open a one-shot TCP client to `addr`, send one NDJSON request with
> the forwarded `method` and `params`, read one line, return the child's `result`
> or propagate its `error` code.

**How it works:** `call` is a thin RPC proxy — no assertion logic yet.

## set-get — Set and get via proxy

> **Hint:** Reuse `spawn` once, then two `call` RPCs on the same `addr`: `set`
> then `get`. The toy returns `hit` and `value` on a hit.

**How it works:** Stage 4 verifies state persists within one spawned child.

## run-case — Run one case

> **Hint:** `run_case` = spawn + call + subset check. Every key in `expect` must
> match the child's `result`; extra result keys are fine. Return `{}` on success,
> `CASE_FAILED` on mismatch.

**How it works:** Subset equality lets you assert partial results (e.g. only
`message` on ping).

## run-suite — Run a suite

> **Hint:** Each `run_case` spawns a **fresh** child — cases are isolated. Run
> three assertions: ping, get miss, set success.

**How it works:** The suite catches harnesses that reuse stale child state across
cases.

## respawn — Spawn again

> **Hint:** Two `spawn` calls must return different addresses. Each child gets its
> own port and data dir.

**How it works:** The tester pings both addresses independently.

## concurrent — Parallel calls

> **Hint:** Mutex around shared maps if needed. Two goroutines call `call` on the
> same spawned `addr` concurrently — both must succeed.

**How it works:** Concurrent `call` RPCs on one harness connection exercise
thread-safe proxying.

## gauntlet — The gauntlet

> **Hint:** Run several `run_case` assertions (ping, get miss, set), then `spawn`
> once and exercise three keys via `call` set/get in one child.

**How it works:** The gauntlet mixes isolated `run_case` assertions with a
stateful multi-call sequence through one spawn.
