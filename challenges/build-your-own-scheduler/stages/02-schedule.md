# Stage 2: Schedule a delayed job

The core loop: enqueue work for the future, workers poll when it's due.

## Your task

Implement **`schedule`** and **`poll`**:

```
→ {"id": "1", "method": "schedule", "params": {"payload": {"task": "hello"}, "delay_ms": 300}}
← {"id": "1", "result": {"job_id": "j-abc"}}

→ {"id": "2", "method": "poll", "params": {}}
← {"id": "2", "result": {"job": null}}   ← not due yet

(after ~300ms)

→ {"id": "3", "method": "poll", "params": {}}
← {"id": "3", "result": {"job": {"job_id": "j-abc", "payload": {"task": "hello"}, "attempt": 1, "lease_token": "..."}}}
```

`poll` is **non-blocking** — return `{"job": null}` immediately when nothing is due.

## Notes

- Use wall-clock time for `delay_ms`.
- The tester checks the job is **not** pollable too early, and **is** pollable
  after the delay (with generous slack for slow CI).
