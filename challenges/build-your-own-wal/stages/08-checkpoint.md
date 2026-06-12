# Stage 8: Snapshots and log truncation

An append-only log grows forever, and recovery replays all of it. Every real
system fixes this the same way: periodically write the **full state** to a
snapshot, then **truncate the log**. Recovery becomes *snapshot + short log
suffix*. This stage is about getting the crash ordering right.

## Your task

Implement the **`checkpoint`** method:

```
→ {"id": "9", "method": "checkpoint", "params": {}}
← {"id": "9", "result": {}}
```

Effects, in this order:

1. Write the entire current state to `<data-dir>/snapshot.json`:
   `{"data": {"key": "value", ...}}` — **atomically**: write to a temp file,
   fsync, then rename over the old snapshot.
2. Only after the snapshot is durable, reset `wal.log` to empty (0 bytes).

Recovery becomes: load `snapshot.json` if present, then replay `wal.log` on
top — **the log wins** for overlapping keys.

## Tests

The tester checkpoints and inspects both files (snapshot must match state
exactly; log must have 0 records), writes more, crashes you, and verifies
the combined recovery. Then it crafts its *own* `snapshot.json` and
`wal.log` with an overlapping key and checks the log took precedence.

## Notes

- Why snapshot-then-truncate and never the reverse? Walk the crash windows:
  crash after the snapshot but before the truncate → the old log replays
  onto the new snapshot. Harmless — `set` and `del` are absolute, so
  replaying already-applied operations is idempotent. Truncate first
  instead, and a crash before the snapshot completes loses *everything not
  yet snapshotted*. One ordering degrades to extra work; the other to data
  loss.
- The atomic-rename trick (temp file + `rename(2)`) is the simplest durable
  publish primitive an OS gives you. You'll reuse it everywhere.
- Don't forget: after truncating, your append handle must point at the new
  empty file, not a deleted inode.
