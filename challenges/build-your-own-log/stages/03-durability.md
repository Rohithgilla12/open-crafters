# Stage 3: Survive a crash

The log is supposed to be the durable source of truth — if it forgets records
on a crash, every consumer downstream is corrupted. Make appends durable.

The on-disk format is yours (the tester never inspects it); an append-only file
is the obvious fit for an append-only log.

## Your task

Persist appends so that after a `SIGKILL` and restart with the same
`--data-dir`:

- every appended record is present at its original offset,
- the next append continues from the correct offset (no gaps, no reuse).

Make an append durable before you reply with its offset.

## Tests

Append three records, crash, restart, read from 0 → all present with the same
offsets. Append again → it gets the next offset. Crash again → everything,
including the post-recovery append, survives.

## Notes

- Append each record to a log file and fsync before acknowledging; replay it on
  boot to rebuild each topic. (Yes — you're building a write-ahead log for your
  log.)
- Restore the per-topic next-offset on recovery, not just the data.
