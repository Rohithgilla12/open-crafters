# Wire Protocol — Build your own MVCC

This document specifies what your program must implement: a transactional
key-value store whose isolation comes from **multi-version concurrency
control**. The tester grades you over TCP, as several clients running
overlapping transactions, and by `SIGKILL`ing your process to check that
committed transactions — and only committed ones — survive.

The on-disk format is **not** graded (that's the WAL challenge's job). The
whole contract is *behavioral*: what a transaction observes, and which commits
are allowed to win.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for your durable state. The tester `SIGKILL`s and
  restarts your process with the same `--data-dir`.

Accept connections within **10 seconds**; handle multiple concurrent ones.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → `UNKNOWN_METHOD`.

## The model: snapshot isolation

Every read and write happens inside a **transaction**. A transaction:

- **captures a snapshot** when it begins — a frozen, consistent view of all
  committed data as of that instant. It reads from that snapshot for its whole
  life, plus its own uncommitted writes (read-your-writes). Commits by *other*
  transactions after it began are invisible to it.
- **buffers its writes** privately until commit; other transactions never see
  them until (and unless) it commits.
- on **commit**, either applies all its writes atomically and becomes visible
  to transactions that begin afterward, or is rejected with `CONFLICT`.

The conflict rule is **first-committer-wins**: a commit is rejected if any key
the transaction wrote was modified by *another* transaction that committed
**after this transaction's snapshot**. That prevents lost updates. It does
**not** prevent write skew (two transactions writing *different* keys) — that
is exactly the line between snapshot isolation and serializability, and you are
building snapshot isolation.

Keys and values are strings.

## Methods

### `ping`
- **params:** `{}` → **result:** `{"message": "pong"}`

### `begin`
- **params:** `{}`
- **result:** `{"txn": "<id>"}` — a server-assigned unique transaction id that
  captures a snapshot as of now.

### `get`
- **params:** `{"txn": "<id>", "key": "<string>"}`
- **result:** `{"value": "<string>", "found": true}` when the key is visible to
  this transaction, else `{"value": null, "found": false}`.
- Visibility = the transaction's own buffered write for the key if any
  (a buffered delete reads as absent), otherwise the newest version committed
  **at or before** this transaction's snapshot.

### `set`
- **params:** `{"txn": "<id>", "key": "<string>", "value": "<string>"}`
- **result:** `{}` — buffers a write in the transaction (not yet visible to
  others).

### `delete`
- **params:** `{"txn": "<id>", "key": "<string>"}`
- **result:** `{}` — buffers a tombstone. Within the transaction the key now
  reads as absent. Like `set`, a delete is a write for conflict purposes.

### `commit`
- **params:** `{"txn": "<id>"}`
- **result:** `{"committed": true}` on success — all the transaction's writes
  become atomically visible to transactions that begin afterward, and are
  durable (see below).
- **error `CONFLICT`** if a key the transaction wrote was committed by another
  transaction after this one's snapshot. The transaction is then finished
  (its id is no longer valid) and **nothing** it wrote is applied.

### `rollback`
- **params:** `{"txn": "<id>"}`
- **result:** `{}` — discard the transaction and all its buffered writes. Its
  id is no longer valid.

A `get`/`set`/`delete`/`commit`/`rollback` naming an unknown or already-finished
transaction → error `UNKNOWN_TXN`.

## Durability

`commit` is the durable event. A committed transaction's writes survive
`SIGKILL` and restart; a transaction that had not committed when the process
died leaves **no** trace. On startup, rebuild every key's version history (and
your commit-sequence counter) from durable state so that snapshots, reads, and
conflict detection behave exactly as before the crash.

In-flight transactions, snapshots, and ids need not survive a crash — only the
set and order of committed transactions.

## A note on fsync

As in the other durability challenges: `SIGKILL` can't drop the OS page cache,
so the tester can't catch a missing `fsync`. Make a commit durable before you
acknowledge it — fsync the commit record before replying `{"committed": true}`.
That discipline is the lesson; the tester checks everything else it can see.
