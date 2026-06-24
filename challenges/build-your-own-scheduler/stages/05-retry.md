# Stage 5: Retries

Real jobs fail. A scheduler retries with backoff (here: fixed delay).

## Your task

Implement **`fail`** with `retry_policy`:

```
schedule {
  "payload": {"n": 1},
  "delay_ms": 0,
  "retry_policy": {"maximum_attempts": 3, "retry_delay_ms": 200}
}
```

1. Poll → `attempt: 1`, call `fail`.
2. After ~200ms, poll → `attempt: 2`.
3. Fail again → `attempt: 3`.
4. Fail a third time → job status `failed`; no more polls.

## Notes

- Retries reschedule at `now + retry_delay_ms`.
- `get_job` should show `status: "failed"` and the error string.
