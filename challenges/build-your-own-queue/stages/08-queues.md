# Stage 8: Many queues and stats

A real broker hosts hundreds of queues — `orders`, `emails`, `audit` — each an
independent stream. You've been keying everything by queue name already, so
this stage is mostly about proving the isolation is airtight, and adding the
one observability method an operator always reaches for: `stats`.

## Your task

**Independent queues.** Every method already takes a `queue`. Operations on one
queue must not touch another: separate ordering, separate in-flight sets,
separate config. A `receipt` is meaningful only within its own queue.

**`stats`** — report a queue's depth, with visibility applied as of now.

```
→ {"id": "1", "method": "stats", "params": {"queue": "orders"}}
← {"id": "1", "result": {"visible": 4, "inflight": 1}}
```

- `visible` — messages a `receive` could return right now.
- `inflight` — messages currently held by a consumer (timeout not yet expired).
- An in-flight message whose visibility timeout has **passed** counts as
  `visible`, not `inflight` — `stats` reflects the same lazy expiry `receive`
  does.
- A queue that has never been used reports `{"visible": 0, "inflight": 0}`.

## Tests

- Fill three queues to different depths (interleaved sends); `stats` reports
  each independent depth, and an unknown queue reports zeroes.
- A `receive` from one queue moves one message to `inflight` there and leaves
  the others' depths untouched.
- A receipt from one queue must **not** ack a message in another (`acked:
  false`), but must still ack its own.
- After receiving with a short timeout, `stats.inflight` is 1; once the timeout
  passes, the same message shows up as `visible` again.

## Notes

- If your state is a map from queue name to a queue object, you're basically
  done — this stage is a checkpoint, not a rewrite.
- Make `stats` run the same "expire overdue in-flight messages" sweep that
  `receive` does, so its numbers never lie about a message whose timeout has
  quietly passed.
