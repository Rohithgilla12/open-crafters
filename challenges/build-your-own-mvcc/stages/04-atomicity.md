# Stage 4: All-or-nothing commits

The "A" in ACID. A transaction that writes three keys must become visible as
*all three or none* — never two-of-three. A reader that catches a transaction
half-applied would see a state that never logically existed (the classic
"money left account A but hasn't arrived in account B"). Multi-version storage
makes this clean: assign all of a transaction's writes the **same** commit
sequence number, in one indivisible step.

## Your task

On `commit`, stamp every buffered write with a single new sequence number and
apply them together. Until that moment, none are visible to anyone else; after
it, all are. `rollback` (and a process that never commits) applies none.

## Tests

- A transaction sets `x`, `y`, `z`. Another open transaction sees none of them
  before commit.
- After commit, a fresh transaction sees `x`, `y`, `z` together.
- A rolled-back multi-key transaction changes nothing, and its new keys stay
  absent.

## Notes

- One sequence number per commit, not per write — that's what makes the writes
  share an atomic visibility point.
- "Visible all at once" is automatic if you bump the global sequence counter
  *after* staging all versions, and readers compare against `seq ≤ snapshot`.
