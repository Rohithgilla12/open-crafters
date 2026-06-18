# Stage 9: The gauntlet

Nothing new to implement — this interleaves appends, offset commits, and
retention across several topics, and `SIGKILL`s you between rounds, checking
every record (at its absolute offset), every `stats`, and every committed offset
against a reference model.

## What the tester does

Each round it appends to multiple topics (asserting the offsets it gets back),
commits a couple of group offsets, occasionally truncates a topic, then crashes
and restarts — and verifies the entire world: each topic's `start_offset`/
`end_offset`, the exact records and absolute offsets in the retained range, and
every group's committed offset.

## Notes

- This is the earlier stages working together: monotonic offsets (2), durability
  (3), per-topic isolation (4), durable group offsets (5, 8), and absolute-offset
  retention (7).
- The classic failures: renumbering offsets after a truncation, losing the
  next-offset or committed offsets across a crash, or replaying recovery events
  out of order.
- Pass this and you've built the log abstraction the entire event-streaming
  world is made of.
