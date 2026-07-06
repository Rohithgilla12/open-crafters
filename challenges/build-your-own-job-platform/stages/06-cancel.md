# Stage 6: Cancel pending

Implement `cancel_job` (proxy to scheduler `cancel`) and `get_job` (return status). Cancelled jobs must never be delivered via `receive_work`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Proxy `cancel` to scheduler; cancelled jobs must never appear in `receive_work`.

Or run: <code>crafters hint job-platform --stage cancel</code>
</details>
<!-- /crafters-stage-hint -->
