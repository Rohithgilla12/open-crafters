# Stage 9: The gauntlet

Nothing new to implement — this stage interleaves everything you've built and
crashes you repeatedly to catch the design shortcuts that survived the happy
path. If your durability, fencing, and visibility logic only work one at a
time, this is where it shows.

## What the tester does

- Sends a few dozen messages spread across several queues.
- Runs rounds of: `receive` with short visibility timeouts, then a random mix
  of `ack`, `nack`, and "leave it in-flight to time out" — across all queues.
- `SIGKILL`s you between every round and reconnects.
- After the last crash, drains every queue and checks the books.

Throughout, it holds you to the core invariant of an at-least-once queue:

- **Nothing acked ever comes back.** If a body you successfully acked is
  redelivered — after a crash, after a timeout, ever — you fail.
- **Nothing is lost.** Every message sent must be delivered until acked; at the
  end, every un-acked message must still be drainable.
- **No message teleports.** A body sent to one queue must never surface in
  another (you didn't configure any dead-letter policy here).

## Notes

- The usual failure here is a durability gap that only opens under
  interleaving: acking in memory but persisting lazily, so a crash resurrects an
  acked message; or persisting a dead-letter move as two non-atomic steps. Make
  every `send`/`ack`/move durable *before* you answer.
- The tester only removes a body from its "still owed" set when **your** `ack`
  returns `true`. If a delivery's timeout lapsed before the ack landed, the ack
  returns `false` (fencing!) and the tester simply expects that message again —
  exactly as a real client would have to cope. Your job is just to never lose
  one and never resurrect an acked one.
- If you pass this, you've built a broker that keeps its promises across
  crashes, slow consumers, and contention. That's the whole job.
