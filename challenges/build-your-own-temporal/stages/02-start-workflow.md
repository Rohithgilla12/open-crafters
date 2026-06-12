# Stage 2: Start a workflow

A workflow engine's first job is bookkeeping: a client asks to start a
workflow, and the engine records the execution and can report on it.

## Your task

Implement two methods:

**`start_workflow`** — register a new workflow execution.

```
→ {"id": "1", "method": "start_workflow", "params": {"workflow_id": "wf-1", "workflow_type": "greet", "input": {"name": "world"}, "task_queue": "default"}}
← {"id": "1", "result": {"run_id": "8f1d2c3e"}}
```

- Generate a unique `run_id` (a UUID is fine).
- `workflow_type` and `input` are opaque to you — store them; workers will
  consume them later.
- If a workflow with the same `workflow_id` already exists, reply with error
  code `WORKFLOW_ALREADY_EXISTS`.

**`describe_workflow`** — report execution state.

```
→ {"id": "2", "method": "describe_workflow", "params": {"workflow_id": "wf-1"}}
← {"id": "2", "result": {"workflow_id": "wf-1", "run_id": "8f1d2c3e", "workflow_type": "greet", "status": "RUNNING", "result": null, "error": null}}
```

- A freshly started workflow has status `RUNNING`.
- Unknown `workflow_id` → error code `WORKFLOW_NOT_FOUND`.

## Notes

- Start building a workflow record now: id, run_id, type, task queue, status.
  Every later stage hangs more state off this record.
- Error responses look like
  `{"id": "...", "error": {"code": "WORKFLOW_NOT_FOUND", "message": "..."}}` —
  the `message` text is up to you, the `code` is checked exactly.
