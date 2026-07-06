# Stage 7: Several jobs

Submit multiple jobs with `delay_ms: 0`. Each distinct payload must be deliverable via `receive_work` and completable.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Dispatch loop must drain all due jobs into queue `jobs`; workers receive in send order.

Or run: <code>crafters hint job-platform --stage multi</code>
</details>
<!-- /crafters-stage-hint -->
