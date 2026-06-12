# Stage 5: Schedule and run activities

Workflows orchestrate; **activities** do the actual work (call an API, send
an email, charge a card). A workflow task's commands can schedule activities;
activity workers poll for them, execute, and report results — which flow back
into the workflow's history and wake it up.

## Your task

Support the **`SCHEDULE_ACTIVITY`** command in `complete_workflow_task`:

```json
{"type": "SCHEDULE_ACTIVITY", "attributes": {"activity_id": "a1", "activity_type": "multiply", "input": {"a": 2, "b": 3}}}
```

Effects: append `ACTIVITY_TASK_SCHEDULED` to history, and make an *activity
task* available on the workflow's task queue.

Implement **`poll_activity_task`** (same non-blocking claim semantics as
workflow tasks):

```
← {"id": "6", "result": {"task": {"task_token": "t-def", "workflow_id": "wf-act", "run_id": "...", "activity_id": "a1", "activity_type": "multiply", "input": {"a": 2, "b": 3}, "attempt": 1}}}
```

Implement **`complete_activity_task`**:

```
→ {"id": "7", "method": "complete_activity_task", "params": {"task_token": "t-def", "result": 6}}
```

Effects: append `ACTIVITY_TASK_COMPLETED` (with `activity_id` and `result`),
and schedule a workflow task — the workflow has a reason to wake up now. The
next workflow task's history will show the full story:

```
WORKFLOW_EXECUTION_STARTED → ACTIVITY_TASK_SCHEDULED → ACTIVITY_TASK_COMPLETED
```

## Tests

The tester also checks that **no workflow task is delivered while the
activity is outstanding** — the workflow is blocked waiting, and there's
nothing for it to decide yet.

## Notes

- Track pending activities per workflow (id → type, input, attempt). The
  `attempt` field is always 1 for now; stage 6 makes it interesting.
