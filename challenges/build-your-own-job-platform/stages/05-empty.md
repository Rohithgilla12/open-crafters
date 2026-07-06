# Stage 5: Nothing to do

When no jobs are due and the queue is empty, `receive_work` returns `{"work": null}` immediately (non-blocking).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** When scheduler has nothing due and queue is empty, return `{"work": null}` without blocking.

Or run: <code>crafters hint job-platform --stage empty</code>
</details>
<!-- /crafters-stage-hint -->
