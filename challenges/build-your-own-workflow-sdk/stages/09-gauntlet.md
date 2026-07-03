# Stage 9: The gauntlet

Nothing new to implement — this stage verifies your **`pipeline`** workflow
orchestrates activity → timer → complete with correct waiting states at
every step.

## Your task

Implement **`pipeline`** (see [PROTOCOL.md](../PROTOCOL.md)) and pass the
full progression:

1. `WORKFLOW_EXECUTION_STARTED` → `SCHEDULE_ACTIVITY` (`step1`)
2. `ACTIVITY_TASK_SCHEDULED` → `[]`
3. `ACTIVITY_TASK_COMPLETED` → `START_TIMER` (`pause`, 100ms)
4. `TIMER_STARTED` → `[]`
5. `TIMER_FIRED` → `COMPLETE_WORKFLOW` (`result: "done"`)

## Notes

- This is the SDK equivalent of integration testing — every waiting state and
  transition must be correct.
- If you've been tracking "last event type" per workflow, this stage catches
  shortcuts that worked for simpler flows.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `pipeline` chains activities and timers. Walk the full history event by event, tracking what's done vs pending. Each replay step asks: "given everything so far, what's the next command?" — maybe schedule, maybe wait, maybe complete.

Or run: <code>crafters hint workflow-sdk --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
