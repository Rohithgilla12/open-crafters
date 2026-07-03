# Walkthrough — Build your own ID generator

Snowflake IDs in nine stages. `crafters hint id-generator` for spoiler-free nudges.

## bind — Boot the server

> **Hint:** Same NDJSON loop as every coordination challenge — `ping` returns pong.

**How it works:** Parse `--port` and `--data-dir`, accept TCP, one JSON line per request.

## next-id — Generate an ID

> **Hint:** Pack `(timestamp - epoch) << 22 | worker_id << 12 | sequence` into a decimal string.

**How it works:** `next_id` allocates; `parse` decodes for tests. Default `worker_id` is 0 until `configure`.

## sortable — Time-ordered IDs

> **Hint:** Never decrease `last_timestamp_ms`; within a ms, increment `sequence`.

**How it works:** Monotonic state machine: new ms → seq 0; same ms → seq++.

## worker-id — Worker partitioning

> **Hint:** Store `worker_id` from `configure` and OR it into every composed ID.

**How it works:** 10 bits in the middle of the 64-bit word; validate 0–1023.

## sequence — Per-millisecond sequence

> **Hint:** A mutex around `(last_ts, last_seq)` — the gauntlet will race you without it.

**How it works:** 5000 rapid calls stay unique by bumping sequence until the millisecond turns.

## clock-skew — Dense millisecond allocation

> **Hint:** For `batch`, capture one `now_ms()` bucket and mint many IDs before re-reading the clock.

**How it works:** Single-RPC batch should produce sequences 0..N-1 at one timestamp when N ≤ 4096.

## batch — Batch allocate

> **Hint:** Loop the same allocator as `next_id`; cap at 1024 per call.

**How it works:** Returned IDs must be strictly increasing integers.

## durability — Survive a crash

> **Hint:** Persist `last_timestamp_ms` and `last_sequence` after each allocation — atomic rename is enough.

**How it works:** After SIGKILL, resume above the last issued ID; never reuse.

## gauntlet — The gauntlet

> **Hint:** One mutex, one state file, no per-connection counters.

**How it works:** 30 workers × 150 IDs must be globally unique; then a throughput mix of `next_id` and `batch`.
