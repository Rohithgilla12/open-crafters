# Stage 3: Schedule an activity

Real workflows rarely complete immediately — they schedule activities, start
timers, and wait for signals. The first step: emit `SCHEDULE_ACTIVITY` when
the workflow decides to delegate work.

## Your task

Implement the **`fetch`** workflow (see [PROTOCOL.md](../PROTOCOL.md)):

When history ends at `WORKFLOW_EXECUTION_STARTED`, return:

```json
{"commands": [{
  "type": "SCHEDULE_ACTIVITY",
  "attributes": {
    "activity_id": "fetch",
    "activity_type": "fetch",
    "input": <copy workflow input from STARTED event>
  }
}]}
```

## Notes

- During replay, seeing `ACTIVITY_TASK_SCHEDULED` in history means the workflow
  already scheduled this activity in a previous task — don't schedule again.
- This stage only tests the "just started" case; the next stage handles
  activity completion.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** For `fetch` at the start, history has only `WORKFLOW_EXECUTION_STARTED`. Return `SCHEDULE_ACTIVITY` — you're at the "need to run side effect" point. Don't complete yet; the activity hasn't run.

Or run: <code>crafters hint workflow-sdk --stage schedule-activity</code>
</details>
<!-- /crafters-stage-hint -->
