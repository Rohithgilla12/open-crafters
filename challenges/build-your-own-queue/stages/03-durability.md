# Stage 3: Survive a crash

An in-memory queue loses everything when the process dies — and processes die.
The point of a *broker* is that a message a producer was told you accepted is
not lost, and a message a consumer acked is not done twice. This stage makes
`send` and `ack` durable.

Unlike the WAL challenge, **the on-disk format is yours** — JSON, a log, a
sqlite file, whatever. The tester never looks inside `--data-dir`. It only
crashes you and checks behavior.

## Your task

Persist enough to `--data-dir` that, after a `SIGKILL` and restart with the
same `--data-dir`:

- every message from an acknowledged `send` is still there (unless acked),
- every `ack`ed message stays gone,
- a message that was **in-flight but never acked** when you crashed comes back
  **visible** — an un-acked message is owed redelivery, never dropped.

You must persist *before* you acknowledge: return the `send` id only once the
message is durable, and report `acked: true` only once the removal is durable.

## Tests

The tester sends `x`, `y`, `z`; receives and acks `x`; receives `y` and leaves
it in-flight; then `SIGKILL`s you. After restart it drains the queue and
expects exactly `[y, z]` (in send order) — `x` is gone, `y` (un-acked) and `z`
survived. It acks both, crashes again, and expects an empty queue.

## Notes

- In-flight is a *runtime* condition, not a durable one. After a crash there
  are no live consumers, so on recovery every un-acked message is simply
  visible again. You do **not** need to persist receipts, visibility timers, or
  `receives` counts.
- Simplest correct approach: keep an append-only log of `send`/`ack` events and
  replay it on boot (you may recognize this from the WAL challenge). A
  periodically-rewritten snapshot of "messages not yet acked" works too. Pick
  whichever you can make crash-safe.
- "Persist before you ack" is the entire lesson. If you ack the network request
  before the write is durable, a crash in the gap silently breaks your
  promise — to the producer (lost message) or the consumer (work redone).
