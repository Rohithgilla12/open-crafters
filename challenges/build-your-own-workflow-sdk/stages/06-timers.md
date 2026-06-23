# Stage 6: Durable timers in replay

On the server you built durable timers that fire after wall-clock delays. In
the SDK, timers appear as history events: `TIMER_STARTED` then `TIMER_FIRED`.
Your replay engine must handle both.

## Your task

Implement **`timer_wait`** (see [PROTOCOL.md](../PROTOCOL.md)):

| Last event in history | Commands |
|---|---|
| `WORKFLOW_EXECUTION_STARTED` | `START_TIMER` with `timer_id: "t1"`, `duration_ms: 500` |
| `TIMER_STARTED` | `[]` (waiting) |
| `TIMER_FIRED` for `t1` | `COMPLETE_WORKFLOW` with `result: "timer fired"` |

## Notes

- Your SDK never waits on a real clock — the tester supplies histories with
  `TIMER_FIRED` already appended. You're deciding what commands the workflow
  *would* emit at each point.
- `duration_ms` in `START_TIMER` is metadata for the server; replay just
  emits the command when the workflow code calls `sleep()`.
