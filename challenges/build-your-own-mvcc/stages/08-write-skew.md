# Stage 8: The isolation boundary

This stage pins down *exactly* how strong your isolation is — and proves you
didn't accidentally build something stronger (which would be wrong here, and
slower). Snapshot isolation prevents lost updates, but it deliberately allows
**write skew**: two transactions that read the same data but write *different*
keys may both commit, even if together they break an invariant.

The textbook example: two on-call doctors each check "is anyone else on call?",
see one other person, and each marks themselves off — both commit, and now
nobody is on call. Snapshot isolation permits this; serializability would not.
You are building snapshot isolation, so both must commit.

## Your task

Nothing new to implement — this is a correctness boundary. Your conflict
detection must fire on write-write conflicts to the **same** key (Stage 5) and
**only** those. Transactions writing disjoint keys must both commit, even when
each read what the other changed.

## Tests

- Seed `x = 0`, `y = 0`.
- `T1` reads `x` and `y`, then writes `x = 1`.
- `T2` reads `x` and `y`, then writes `y = 1`.
- Both commit (write skew is allowed under snapshot isolation). Final state is
  `x = 1`, `y = 1`.

## Notes

- If this stage fails with an unexpected `CONFLICT`, your conflict check is too
  aggressive — it's keying on reads, or on *any* concurrent commit, instead of
  on a concurrent commit **to a key you wrote**.
- Preventing write skew requires tracking the read set and doing serializable
  validation (SSI) — a different, heavier algorithm. That's not this challenge;
  resist the urge.
