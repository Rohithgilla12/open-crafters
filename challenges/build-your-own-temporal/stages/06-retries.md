# Stage 6: Activity retries with backoff

Activities fail — networks flake, services 503. A workflow engine's job is to
absorb transient failure so workflow authors don't write retry loops by hand.
Retries are a *server-side* concern: the worker just reports "this attempt
failed", and the server decides whether and when to try again.

## Your task

Support an optional **`retry_policy`** on `SCHEDULE_ACTIVITY`:

```json
{"type": "SCHEDULE_ACTIVITY", "attributes": {"activity_id": "a1", "activity_type": "always-fails", "input": null, "retry_policy": {"maximum_attempts": 3, "initial_interval_ms": 200, "backoff_coefficient": 2.0}}}
```

Implement **`fail_activity_task`**:

```
→ {"id": "8", "method": "fail_activity_task", "params": {"task_token": "t-def", "error": "transient failure"}}
```

Semantics, where `n` is the attempt that just failed (1-based):

- **`n < maximum_attempts`** — schedule a retry: the activity task becomes
  available again after `initial_interval_ms × backoff_coefficient^(n-1)`
  milliseconds, with `attempt = n + 1`. **No history event is recorded** —
  retried failures are invisible to the workflow, exactly like in Temporal.
- **`n == maximum_attempts`** — retries exhausted: append
  `ACTIVITY_TASK_FAILED` (with `activity_id` and `error`) and schedule a
  workflow task so the workflow can react.

Also support the **`FAIL_WORKFLOW`** command (the workflow's reaction might
be to give up): append `WORKFLOW_EXECUTION_FAILED`, set status `FAILED`,
expose the error via `describe_workflow`.

Default when `retry_policy` is omitted: `maximum_attempts: 1` (fail
immediately becomes permanent).

## Tests

With the policy above, the tester fails the activity three times and checks:

- attempts are delivered as 1, 2, 3 — with **at least** ~200ms before
  attempt 2 and ~400ms before attempt 3 (delivering retries immediately
  fails the stage),
- history contains exactly **one** `ACTIVITY_TASK_FAILED` event — the final
  one.

## Notes

- "Available again after a delay" is best modeled as an `available_at`
  timestamp on the pending activity; `poll_activity_task` skips entries whose
  time hasn't come. No background scheduler needed for this stage.
