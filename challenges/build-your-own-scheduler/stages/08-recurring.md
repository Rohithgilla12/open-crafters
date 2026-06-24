# Stage 8: Recurring jobs

Cron's building block: after success, schedule the next run.

## Your task

Implement `interval_ms` on **`schedule`**:

```
schedule {"payload": {"tick": 1}, "delay_ms": 0, "interval_ms": 300}
```

1. Poll → complete.
2. After ~300ms, poll → new job (new `job_id`) with same payload.
3. Complete again → another run ~300ms later.

Recurring stops when a run **fails** (retries exhausted) or is cancelled.
