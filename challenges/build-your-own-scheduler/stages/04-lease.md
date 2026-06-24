# Stage 4: Job leases

Like a message queue visibility timeout: a polled job is **leased** — other
polls must not see it until the lease expires.

## Your task

When a worker polls a job, hide it for `lease_ms` (default 3000; the tester
uses `lease_ms: 400` on `schedule`).

1. Poll a due job → get `lease_token`.
2. Poll again immediately → `{"job": null}`.
3. Wait for the lease to expire (~400ms + slack).
4. Poll again → same logical job, **new** `lease_token`, same `attempt`.

## Notes

- This prevents two workers from processing the same run concurrently.
- Compare with the queue challenge's visibility timeout — same idea, time axis.
