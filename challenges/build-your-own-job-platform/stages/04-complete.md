# Stage 4: Complete work

`receive_work` returns `lease_token` and `receipt`. `complete_work` must call scheduler `complete` and queue `ack` so the job status becomes `completed`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `complete_work` calls scheduler `complete` with `lease_token` then queue `ack` with `receipt`.

Or run: <code>crafters hint job-platform --stage complete</code>
</details>
<!-- /crafters-stage-hint -->
