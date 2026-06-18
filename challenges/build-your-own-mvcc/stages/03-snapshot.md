# Stage 3: Snapshot isolation

Here's the idea the whole challenge is named for. A transaction shouldn't see
the world shifting under its feet: if it read `balance = 100` a moment ago,
reading it again should still say `100`, even if someone else just committed a
change. The fix is **multi-version** storage — keep old versions around so each
transaction can read a consistent snapshot frozen at the instant it began.

## Your task

Give every key a **version history** instead of a single value. Stamp each
committed write with a monotonically increasing **commit sequence number**. When
a transaction begins, record the current sequence number as its **snapshot**. A
`get` then returns, for that key, the newest version whose sequence number is
**≤ the transaction's snapshot** (overlaid by the transaction's own buffered
writes).

A commit by another transaction *after* a reader began has a higher sequence
number, so it's simply not visible to that reader. No locks, no blocking — just
"read the version you're allowed to see."

## Tests

- A reader begins and sees `k = v0`.
- A concurrent writer commits `k = v1`.
- The reader still sees `v0`, repeatedly (its snapshot is frozen).
- A transaction begun *after* the writer's commit sees `v1`.

## Notes

- Store per key a list of `(seq, value)` in increasing `seq` order; a read is
  "the last entry with `seq ≤ snapshot`."
- The snapshot is just an integer captured at `begin`. That integer is the
  entire mechanism — internalize that and the rest of MVCC falls out.
- Don't garbage-collect old versions yet; correctness first.
