# Stage 7: Survive a crash

A committed transaction is a promise. If the process dies a millisecond later,
that data must still be there on restart — and a transaction that *hadn't*
committed must leave no trace. This stage makes commits durable.

As in the message-queue challenge, the **on-disk format is yours** — an
append-only log of committed transactions, a snapshot file, anything. The tester
never looks inside `--data-dir`; it only crashes you and checks behavior.

## Your task

Persist enough that, after a `SIGKILL` and restart with the same `--data-dir`:

- every committed transaction's writes are present (with correct version
  ordering and visibility),
- a transaction that was open-but-uncommitted at crash time left nothing,
- new commits made after recovery are durable too.

Make a commit durable **before** you reply `{"committed": true}`.

## Tests

The tester commits `d1`, leaves a second transaction open with an uncommitted
`d2`, then `SIGKILL`s you. After restart, `d1` is present and `d2` is absent. It
then commits `d3`, crashes again, and expects `d3` to survive.

## Notes

- The simplest correct approach: append each committed transaction (its writes
  + assigned sequence number) to a log and fsync before acknowledging; on boot,
  replay the log to rebuild every key's version list and restore the sequence
  counter. You may recognize this shape from the WAL challenge.
- Restore the **commit-sequence counter** on recovery, not just the data — the
  next commit must get a higher number, or conflict detection breaks.
- In-flight transactions don't survive a crash, and shouldn't: nothing
  acknowledged them.
