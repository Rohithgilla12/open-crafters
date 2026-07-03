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
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `schedule` with `interval_ms` reschedules itself after `complete`: next `run_at_ms = now + interval_ms`, same payload, new lease cycle. The job id may stay the same or spawn successors per your protocol.

Or run: <code>crafters hint scheduler --stage recurring</code>
</details>
<!-- /crafters-stage-hint -->
