# Stage 6: Cancel a job

Pending work should be cancellable before it runs.

## Your task

Implement **`cancel`**:

```
schedule {"payload": "later", "delay_ms": 10000}
cancel {"job_id": "..."}  → {"cancelled": true}
poll → never returns that job
get_job → status "cancelled"
```

Cancelling an unknown job → `JOB_NOT_FOUND`. Cancelling a completed job →
`{"cancelled": false}`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `cancel(job_id)` removes a pending job or stops a not-yet-completed scheduled job. Return whether it existed. Completed jobs can't be cancelled.

Or run: <code>crafters hint scheduler --stage cancel</code>
</details>
<!-- /crafters-stage-hint -->
