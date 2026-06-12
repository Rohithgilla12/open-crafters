# Stage 9: The gauntlet

The final stage. Nothing new to implement — this is where shortcuts that
survived eight happy-path stages come to die. The tester runs a seeded,
randomized workload and crashes you over and over, checking your answers
against its own model of the truth the whole way.

## Tests

Four rounds, each: **25 random operations** (sets with fresh values, deletes
of possibly-missing keys — the `deleted` flag is checked against the model
every time), a mid-round **`checkpoint`** from round 2 on, a full state
verification, a **SIGKILL**, and a full re-verification after recovery.

Then the kicker: with your process stopped, the tester **parses your
`snapshot.json` and `wal.log` directly** and reconstructs the state they
imply, per spec. It must agree with the model on every key — your served
state and your durable bytes are not allowed to drift, ever.

## Notes

- The run is seeded and deterministic: a failure reproduces exactly, every
  time. Read the error — it names the round, the key, and what diverged.
- Classic corpses found here: a stale append handle after checkpoint (writes
  silently lost to a deleted inode), `deleted: true` for missing keys, lock
  gaps between "append to log" and "update memory", snapshot written
  non-atomically.
- If this passes: you've built a real write-ahead log — framing, prefix
  recovery, corruption detection, compaction, and the fsync discipline that
  makes "OK" mean something. The same machinery, scaled up, is the bottom
  layer of Postgres, etcd, and Kafka. Next up, a natural sequel: *Build your
  own Temporal* runs this exact durability contract one level up the stack.
