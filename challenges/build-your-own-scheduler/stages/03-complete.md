# Stage 3: Complete a job

Workers finish by reporting success with the lease token from `poll`.

## Your task

Implement **`complete`** and **`get_job`**:

```
→ complete {"lease_token": "...", "result": "done"}
→ get_job {"job_id": "..."}
← status "completed"
```

After `complete`, `poll` must not return that job again.

## Notes

- Only the holder of a valid `lease_token` may complete.
- Expired leases → error code `LEASE_NOT_FOUND`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `poll` hands out a due job with a `lease_token`. `complete` takes that token and marks the job `completed` with a result. Invalid or stale tokens are rejected.

Or run: <code>crafters hint scheduler --stage complete</code>
</details>
<!-- /crafters-stage-hint -->
