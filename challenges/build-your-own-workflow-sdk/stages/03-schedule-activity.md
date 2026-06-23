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
