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
