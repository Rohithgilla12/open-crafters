# Stage 6: Deletes and tombstones

Deleting a key in an MVCC store isn't "remove the value" — older snapshots must
still see the value that was there *for them*. So a delete is just another
version: a **tombstone** that says "as of this sequence number, the key is
gone." Readers with an earlier snapshot sail right past it.

## Your task

Implement `delete`: buffer a tombstone (a write whose "value" is *absent*, not
the empty string). Within the transaction the key now reads as not found. On
commit it becomes a versioned tombstone like any other write — and counts as a
write for conflict detection.

A read resolves to "found" only if the newest visible version is a real value,
"not found" if it's a tombstone (or there's no visible version).

## Tests

- Delete within a transaction → the key reads absent for that transaction;
  after commit, later transactions see it absent.
- Re-setting a deleted key restores it.
- A reader's earlier snapshot still sees the value after a concurrent delete
  commits.
- `delete` vs a concurrent `set` of the same key is a write-write `CONFLICT`,
  just like two sets.

## Notes

- Keep tombstone distinct from the empty-string value `""` — `""` is a real,
  found value.
- Don't special-case deletes in conflict detection: a tombstone is a write, so
  the "newest version seq > snapshot" rule already covers delete-vs-write.
