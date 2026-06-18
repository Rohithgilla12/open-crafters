# Stage 5: Write-write conflicts

Snapshot isolation has a famous failure mode if you stop here: the **lost
update**. Two transactions read `count = 10`, both compute `11`, both write,
both commit — and one increment silently vanishes. Snapshot isolation forbids
this with the **first-committer-wins** rule: if you try to commit a write to a
key that someone else committed *after your snapshot*, you lose — your whole
transaction is rejected with `CONFLICT`.

## Your task

At `commit`, for each key the transaction wrote, check the key's version
history: if its newest committed version has a sequence number **greater than
this transaction's snapshot**, abort the entire commit with error code
`CONFLICT` and apply nothing. Otherwise commit normally.

```
← {"id":"9","error":{"code":"CONFLICT","message":"key \"k\" was modified by a concurrent transaction"}}
```

## Tests

- Two transactions begin, both write `k`. The first commit succeeds; the second
  returns `CONFLICT`, and the first writer's value stands.
- Transactions writing **different** keys both commit (no false conflicts).
- A transaction whose snapshot already includes the latest write of `k` may
  write `k` and commit cleanly — it isn't concurrent with anyone.

## Notes

- The check is per *written* key only. A read-only transaction never conflicts.
- "Newest version's seq > my snapshot" is the whole test — you already have the
  version list and the snapshot from the previous stages.
- An aborted transaction must leave **no** versions behind, and its id becomes
  invalid (a later `commit`/`get` on it is `UNKNOWN_TXN`).
