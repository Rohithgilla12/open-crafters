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
