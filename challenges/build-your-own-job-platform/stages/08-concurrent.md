# Stage 8: Parallel submits

Many concurrent `submit_job` calls must all produce unique `job_id`s and eventually be receivable.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Scheduler generates unique `job_id`s; your gateway can be stateless aside from the dispatch lock.

Or run: <code>crafters hint job-platform --stage concurrent</code>
</details>
<!-- /crafters-stage-hint -->
