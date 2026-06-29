# Walkthrough — Build your own MVCC

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint mvcc` prints just the hint for your next stage;
`crafters walkthrough mvcc --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Newline-delimited JSON server loop — read line, dispatch on
> `method`, respond, flush. `ping` returns `pong`. Build transport once;
> transactions come next.

**How it works:** Each connection is independent. The reference separates wire
handling from storage logic. `--data-dir` is parsed early for later durability.

## transactions — Begin, commit, rollback

> **Hint:** `begin` returns a transaction id. `set`/`get`/`del` inside a txn
> are buffered, not visible to others until `commit`. `rollback` discards the
> buffer. Outside a txn, reads see only committed data.

**How it works:** Active transactions hold pending writes in a per-txn map.
`get` in a txn reads from the txn buffer first, then committed state.
`commit` merges the buffer atomically; `rollback` drops it.

## snapshot — Snapshot isolation

> **Hint:** On `begin`, record the current committed sequence number — that's
> the txn's snapshot. All reads in the txn see data as of that sequence, even
> if other txns commit while this one is open.

**How it works:** The reference stores a monotonic `commit_seq`. Each txn
captures `snapshot_seq` at begin. Reads resolve the latest version with
`seq <= snapshot_seq`. Concurrent commits don't affect in-flight reads.

## atomicity — All-or-nothing commits

> **Hint:** A commit applies every write in the txn together or not at all.
> Partial commits are forbidden — one `commit` RPC, one new sequence number,
> all keys updated together.

**How it works:** Commit validates conflicts, then assigns one new sequence and
appends all writes at that sequence. Readers at later sequences see the full
set; readers at the snapshot see none of it until commit succeeds.

## conflicts — Write-write conflicts

> **Hint:** If txn A read key K at snapshot S, and txn B committed a write to K
> after S before A commits, A's commit must fail with `CONFLICT`. First
> committer wins on overlapping keys.

**How it works:** On commit the reference checks every key the txn wrote: if any
key has a committed version with `seq > snapshot_seq`, return `CONFLICT`.
Otherwise assign the next sequence. This is first-committer-wins MVCC.

## deletes — Deletes and tombstones

> **Hint:** `del` in a txn records a tombstone, not absence. Committed deletes
> hide the key from future reads at that sequence and later. `get` returns
> `found: false` for tombstoned keys.

**How it works:** Versions store `nil` or a sentinel for deleted keys. Reads
walk versions newest-first and stop at the first version `<= snapshot_seq`.
A tombstone at seq N hides older values for snapshots `>= N`.

## durability — Survive a crash

> **Hint:** Append each committed transaction to a durable log under
> `--data-dir` before acknowledging `commit`. On startup replay commits in
> sequence order to rebuild version chains.

**How it works:** The reference logs commit records (seq, key, value/tombstone),
fsyncs, then returns success. Recovery replays the log into the in-memory
version store. In-flight txns are lost — only committed work survives.

## write-skew — The isolation boundary

> **Hint:** Snapshot isolation prevents write-write conflicts on keys you touch,
> but two txns can each read disjoint keys and commit conflicting invariants.
> This challenge tests that your conflict detection covers keys you *wrote*, not
> keys you only read — know what SI guarantees and what it doesn't.

**How it works:** The reference implements snapshot isolation (not serializable).
Write-write conflicts on overlapping write sets are rejected. Read-only
observations of stale data across txns is allowed by SI — the stage verifies
your conflict rules match the protocol's `CONFLICT` semantics on writes.

## gauntlet — The gauntlet

> **Hint:** Interleave begins, reads, commits, rollbacks, and crashes. The
> invariants: snapshot reads, atomic commits, conflict detection on overlapping
> writes, durable commit log.

**How it works:** The gauntlet stresses concurrent txns and recovery. The
reference never exposes uncommitted writes, never loses committed writes after
replay, and rejects conflicting commits consistently.
