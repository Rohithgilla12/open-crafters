# Stage 2: Replay to completion

In Temporal, a worker receives a workflow task containing the **full event
history** and runs workflow code from scratch. The code replays every past
event (without re-executing side effects) and emits **commands** for what
should happen next.

This stage implements the core RPC and the simplest workflow.

## Your task

Implement **`replay`**:

```
→ {"id": "1", "method": "replay", "params": {
     "workflow_type": "greet",
     "history": [
       {"event_id": 1, "type": "WORKFLOW_EXECUTION_STARTED",
        "attributes": {"workflow_type": "greet", "input": {"name": "world"}}}
     ]
   }}
← {"id": "1", "result": {"commands": [
     {"type": "COMPLETE_WORKFLOW",
      "attributes": {"result": {"greeting": "hello world"}}}
   ]}}
```

Rules the tester enforces:

1. Unknown `workflow_type` → error code `WORKFLOW_TYPE_NOT_FOUND`.
2. If the last event is `WORKFLOW_EXECUTION_COMPLETED` or
   `WORKFLOW_EXECUTION_FAILED`, return `{"commands": []}` — the workflow is
   done.
3. See [PROTOCOL.md](../PROTOCOL.md) for the `greet` workflow definition.

## Notes

- Events in `history` are **facts** — they already happened. Don't re-emit
  commands for past events; only return what the workflow would decide **now**.
- This is the same `greet` workflow from the Temporal challenge, but you're
  implementing the worker logic, not the server.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `replay` takes a workflow type and event history. Walk events in order; for `greet`, emit one `COMPLETE_WORKFLOW` command with the greeting. Same history in → same commands out.

Or run: <code>crafters hint workflow-sdk --stage simple-complete</code>
</details>
<!-- /crafters-stage-hint -->
