# Stage 5: Waiting means empty commands

This is the subtle insight from Temporal stage 4, now on the SDK side: when
a workflow has nothing to do until a new event arrives, it emits **no
commands** — not a completion, not an error, just `{"commands": []}`.

## Your task

Handle waiting states correctly:

1. **`fetch`**: history ends at `ACTIVITY_TASK_SCHEDULED` (activity not yet
   completed) → empty commands.
2. **`signal_wait`**: history ends at `WORKFLOW_EXECUTION_STARTED` only →
   empty commands (waiting for a signal).

## Notes

- Empty commands ≠ workflow complete. The workflow is still `RUNNING`; it's
  just blocked until history grows.
- Compare with stage 2: terminal histories (`WORKFLOW_EXECUTION_COMPLETED`)
  also return empty commands, but for a different reason — the workflow is
  finished.
