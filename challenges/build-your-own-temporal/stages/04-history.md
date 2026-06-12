# Stage 4: Append-only event history

Temporal's superpower is that a workflow's entire life is an **append-only
event log**. Crash recovery, replay, debugging, audit — all of it falls out
of getting the history right. This stage pins down the rules.

## Your task

Implement **`get_history`**:

```
→ {"id": "5", "method": "get_history", "params": {"workflow_id": "wf-2"}}
← {"id": "5", "result": {"events": [{"event_id": 1, "type": "WORKFLOW_EXECUTION_STARTED", "attributes": {...}}, {"event_id": 2, "type": "WORKFLOW_EXECUTION_COMPLETED", "attributes": {"result": 7}}]}}
```

Rules the tester enforces:

1. `event_id` is sequential, starting at **1**, no gaps, per execution.
2. History is append-only and ordered: `WORKFLOW_EXECUTION_STARTED` first,
   terminal events (`WORKFLOW_EXECUTION_COMPLETED`) last.
3. Completing a workflow task with an **empty `commands` list** is valid: it
   means "the workflow is waiting for something". The workflow stays
   `RUNNING`, and — critically — you must **not** deliver another workflow
   task until a *new* event gives the workflow a reason to wake up.
4. Unknown workflow → `WORKFLOW_NOT_FOUND`.

## Notes

- Rule 3 is the subtle one. Track a per-workflow "needs a workflow task" flag:
  set it when a wake-up event is appended (for now, only
  `WORKFLOW_EXECUTION_STARTED`), clear it when a task is claimed. If the flag
  is clear, polling returns `{"task": null}` — no matter how many times the
  worker asks.
- This flag-based design pays off in every remaining stage: activities,
  timers, and signals all just set the flag.
