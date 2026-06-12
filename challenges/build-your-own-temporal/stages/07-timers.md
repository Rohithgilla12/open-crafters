# Stage 7: Durable timers

`await sleep(30 * DAYS)` inside a workflow — that's the demo that sells
durable execution. The workflow code *appears* to block for a month, but no
process is actually waiting: the server records a timer, the worker moves on,
and when the timer fires the workflow gets woken up with a `TIMER_FIRED`
event.

## Your task

Support the **`START_TIMER`** command:

```json
{"type": "START_TIMER", "attributes": {"timer_id": "t1", "duration_ms": 500}}
```

Effects:

1. Append `TIMER_STARTED` (with `timer_id` and `duration_ms`) to history.
2. After `duration_ms` elapses, append `TIMER_FIRED` (with `timer_id`) and
   schedule a workflow task.

## Tests

The tester starts a 500ms timer and verifies:

- no workflow task is delivered while the timer is pending (the workflow has
  nothing to decide yet),
- the `TIMER_FIRED` workflow task arrives **no earlier than** the timer
  duration,
- history reads `WORKFLOW_EXECUTION_STARTED → TIMER_STARTED → TIMER_FIRED`.

## Notes

- Store an absolute fire-at timestamp (`now + duration_ms`), not a countdown.
  This matters enormously in the next stage, where your process gets killed
  while a timer is pending.
- A simple background ticker (check all pending timers every ~50ms) is
  perfectly adequate. Resist the urge for a fancy timer wheel — that's not
  what this stage grades.
