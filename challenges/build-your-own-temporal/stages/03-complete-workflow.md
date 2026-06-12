# Stage 3: Dispatch and complete a workflow task

Here's the heart of the Temporal model: **the server never runs workflow
code**. Workers poll the server for *workflow tasks*, run the user's workflow
function against the delivered history, and report back *commands* —
decisions like "schedule this activity" or "the workflow is done".

In this stage you'll implement that loop for the simplest possible workflow:
one that completes immediately.

## Your task

When a workflow starts, record a `WORKFLOW_EXECUTION_STARTED` event in its
**history** and make a *workflow task* available on its task queue.

**`poll_workflow_task`** — workers fetch tasks. Non-blocking: reply
`{"task": null}` when nothing is available.

```
→ {"id": "3", "method": "poll_workflow_task", "params": {"task_queue": "default"}}
← {"id": "3", "result": {"task": {"task_token": "t-abc", "workflow_id": "wf-1", "run_id": "8f1d2c3e", "workflow_type": "greet", "history": [{"event_id": 1, "type": "WORKFLOW_EXECUTION_STARTED", "attributes": {"workflow_type": "greet", "input": {"name": "world"}}}]}}}
```

- `task_token` is an opaque string you generate; it identifies this claim.
- A claimed task must **not** be delivered to another poller.

**`complete_workflow_task`** — workers report commands.

```
→ {"id": "4", "method": "complete_workflow_task", "params": {"task_token": "t-abc", "commands": [{"type": "COMPLETE_WORKFLOW", "attributes": {"result": {"greeting": "hello world"}}}]}}
← {"id": "4", "result": {}}
```

- `COMPLETE_WORKFLOW` sets the workflow's status to `COMPLETED` and stores the
  result (visible via `describe_workflow`).
- An unknown or already-used `task_token` → error code `TASK_NOT_FOUND`.

## Notes

- This is the stage where the architecture clicks into place: *events* flow
  from server to worker (as history), *commands* flow back. Everything later
  is more event types and more command types.
- Keep history as an ordered list on the workflow record, `event_id` starting
  at 1.
