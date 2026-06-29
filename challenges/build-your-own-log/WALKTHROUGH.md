# Walkthrough — Build your own log

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint log` prints just the hint for your next stage;
`crafters walkthrough log --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Newline-delimited JSON over TCP — one request line in, one response
> line out, flush every time. `ping` returns `pong`. Handle each connection
> independently.

**How it works:** Transport and dispatch are separated. Concurrent connections
each get their own read loop. `--data-dir` is ignored until durability.

## append-read — Append and read by offset

> **Hint:** Each topic is an append-only array with a monotonic end offset.
> `append` returns the offset assigned; `read` takes `(topic, offset, max)` and
> returns values starting at that offset plus `next_offset`. Reads never consume
> data.

**How it works:** The reference keeps per-topic slices and a running end offset.
`read` is pure replay — callers can re-read the same range. `OUT_OF_RANGE` when
offset is past the end. Offsets are absolute from topic creation (0-based).

## durability — Survive a crash

> **Hint:** Persist every append (and later every offset commit) to `--data-dir`
> before acknowledging. On startup replay the log into in-memory topics. Unacked
> appends must not exist; acked appends must survive `SIGKILL`.

**How it works:** The reference appends mutation events to a durable log or
snapshot file, fsyncs, then returns. Recovery rebuilds topic state from disk.
The invariant is write-before-ack for every append.

## topics — Independent topics

> **Hint:** `map[topicName]*topic` — each topic has its own value array and
> offset space. Topic names are opaque; no ordering across topics.

**How it works:** `append`/`read`/`stats` all take a `topic` parameter. Topics
are created lazily on first append. End offsets are per-topic and independent.

## consumer-groups — Consumer group offsets

> **Hint:** Track `(group, topic) → committed_offset` separately from the log.
> `commit_offset` advances the cursor; `committed_offset` reads it. Consumers
> resume from committed + 1.

**How it works:** Consumer state is a nested map. Committing offset N means "I've
processed through N." Reads still start at any offset the client chooses — the
group offset is advisory for resume semantics.

## replay — Replay and batching

> **Hint:** `read` with `max > 1` returns up to `max` values in one RPC.
> `next_offset` tells the client where to continue. Same data can be read again
> — reads are idempotent.

**How it works:** The reference slices `values[offset:end]` capped by `max`.
`next_offset` is the offset after the last returned value (or end if exhausted).
Batching is just a loop bound, not a destructive consume.

## retention — Retention keeps offsets absolute

> **Hint:** `truncate(topic, before)` drops entries with offset `< before` but
> **does not renumber** surviving offsets. Raise `base` to `before` and trim
> the slice — offset 100 must still mean offset 100 after truncation.

**How it works:** Each topic stores `base` (offset of `values[0]`) plus the
slice. Truncate removes old entries and sets `base = before`. `read` maps
absolute offset to `values[offset - base]`. Consumer commits remain valid
because offsets never shift.

## offset-durability — Durable offsets and resume

> **Hint:** Persist committed offsets alongside topic data. After restart,
> `committed_offset` returns the same value and consumers can resume exactly
> where they left off.

**How it works:** The reference includes consumer-group state in the same
snapshot or event log as topic data. Recovery restores both. Commit-before-ack
applies to offset commits too.

## gauntlet — The gauntlet

> **Hint:** Compose everything: durable appends, absolute offsets, retention
> with a rising base, consumer groups, batch reads. Never renumber offsets;
> persist on every mutation.

**How it works:** The gauntlet interleaves appends, reads, truncates, commits,
kills, and restarts. The reference uses the same recovery path throughout —
absolute offsets and durable commits are the invariants that must hold under chaos.
