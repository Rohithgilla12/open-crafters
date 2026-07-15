# Stage 5: Enforce limits

When `take` denies, return `queued=false` / `rate_limited=true` and do **not** enqueue. A following `receive` must not see the denied notification.
