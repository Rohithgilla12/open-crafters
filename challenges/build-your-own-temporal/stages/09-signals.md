# Stage 9: Signals

Workflows often need input from the outside world mid-flight: a human
approves a request, a payment webhook lands, an order gets cancelled.
**Signals** deliver asynchronous events *into* a running workflow — recorded
in history like everything else.

## Your task

Implement **`signal_workflow`**:

```
→ {"id": "9", "method": "signal_workflow", "params": {"workflow_id": "wf-sig", "signal_name": "approve", "input": {"by": "alice"}}}
← {"id": "9", "result": {}}
```

Effects: append `WORKFLOW_EXECUTION_SIGNALED` (with `signal_name` and
`input`) to history, and schedule a workflow task — a signal is a wake-up
event.

Errors:

- Unknown workflow → `WORKFLOW_NOT_FOUND`.
- Workflow already `COMPLETED` or `FAILED` → `WORKFLOW_CLOSED`. You can't
  signal the dead.

## Tests

The tester starts a workflow whose first workflow task completes with **no
commands** — the workflow is idle, waiting. Then it signals, and expects a
workflow task whose history ends with the `WORKFLOW_EXECUTION_SIGNALED`
event.

## Notes

- If you built the "needs a workflow task" flag in stage 4, this stage is
  ~15 lines. That's the payoff of the event-driven design: every new feature
  is "append an event, set the flag".
