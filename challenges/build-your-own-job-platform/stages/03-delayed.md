# Stage 3: Delayed delivery

Jobs with `delay_ms` must not appear in `receive_work` until their fire time. Implement the dispatcher loop: under lock `dispatcher`, poll due jobs from the scheduler and send them to queue `jobs`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Under lock `dispatcher`, poll scheduler until `job: null`, `send` each to queue `jobs` as JSON body, then `receive` from queue in `receive_work`.

Or run: <code>crafters hint job-platform --stage delayed</code>
</details>
<!-- /crafters-stage-hint -->
