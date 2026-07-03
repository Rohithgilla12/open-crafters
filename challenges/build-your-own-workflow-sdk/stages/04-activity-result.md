# Stage 4: React to activity completion

When an activity finishes, the server appends `ACTIVITY_TASK_COMPLETED` to
history and delivers a new workflow task. On replay, your engine sees that
event and decides what to do next — without re-running the activity.

## Your task

Extend **`fetch`**: when history ends at `ACTIVITY_TASK_COMPLETED` for
`activity_id: "fetch"`, emit `COMPLETE_WORKFLOW` with the activity's
`result` as the workflow result.

Example history:

```json
[
  {"event_id": 1, "type": "WORKFLOW_EXECUTION_STARTED", ...},
  {"event_id": 2, "type": "ACTIVITY_TASK_SCHEDULED", "attributes": {...}},
  {"event_id": 3, "type": "ACTIVITY_TASK_COMPLETED",
   "attributes": {"activity_id": "fetch", "result": {"status": 200, "body": "ok"}}}
]
```

Expected commands:

```json
[{"type": "COMPLETE_WORKFLOW", "attributes": {"result": {"status": 200, "body": "ok"}}}]
```

## Notes

- The tester checks you do **not** re-schedule the activity when it's already
  in history.
- Activity side effects happened in the real world; replay only reads the
  recorded result.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** When history ends with `ACTIVITY_TASK_COMPLETED`, the workflow should consume that result and emit `COMPLETE_WORKFLOW`. Replay simulates "what would the code do after the activity returns?"

Or run: <code>crafters hint workflow-sdk --stage activity-result</code>
</details>
<!-- /crafters-stage-hint -->
