# Stage 2: Begin, commit, rollback

A transaction is a unit of work that is invisible to everyone else until it
commits — and then becomes visible all at once. That boundary is the whole
point: it's what lets a database show every reader a coherent picture even
while writers are mid-flight. This stage builds the boundary; later stages make
it correct under concurrency and crashes.

## Your task

Implement `begin`, `get`, `set`, `commit`, and `rollback`.

```
→ {"id":"1","method":"begin","params":{}}
← {"id":"1","result":{"txn":"t-7"}}

→ {"id":"2","method":"set","params":{"txn":"t-7","key":"a","value":"1"}}
← {"id":"2","result":{}}

→ {"id":"3","method":"get","params":{"txn":"t-7","key":"a"}}
← {"id":"3","result":{"value":"1","found":true}}        (read-your-writes)
```

- **`begin`** returns a unique transaction id.
- **`set`/`delete`** buffer writes *privately* — invisible to other
  transactions until commit.
- **`get`** returns the transaction's own buffered write if any, else the
  latest committed value (`{"value": null, "found": false}` when absent).
- **`commit`** makes the transaction's writes visible to transactions begun
  afterward.
- **`rollback`** throws the transaction away.

## Tests

- A transaction sees its own uncommitted writes (including overwrites).
- Another open transaction does **not** see them before commit.
- After commit, a transaction begun afterward sees the writes.
- Rolled-back writes vanish.

## Notes

- Keep two things per open transaction: a private map of buffered writes, and
  (soon) the snapshot it reads from. For this stage "latest committed" is fine;
  the next stage pins down *which* committed version a transaction may see.
- `get` returning your own buffered write is "read-your-writes" — non-negotiable
  and easy to forget once snapshots arrive.
