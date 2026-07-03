# Stage 6: Lease expiry

Locks must not be held forever if a worker crashes without releasing. Leases
bound how long exclusivity lasts.

## Your task

Treat a lock as **free** when `now >= expires_at_ms`. No background timer is
required — check expiry on every operation.

The tester acquires with `lease_ms: 300`, waits ~400ms, then acquires with a
different `holder_id` — which must succeed.

## What the tester checks

- After the short lease elapses, a new `acquire` succeeds.
- `status` shows the new holder.

## Notes

- Lazy expiry (check on access) is enough and matches how renew/durability work.
- `expires_at_ms` is absolute wall-clock time, not relative to process start.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Never run a sweeper. On every acquire/status/release/renew, treat `expires_at_ms <= now` as "free". Wall-clock `expires_at_ms` makes downtime count toward expiry automatically.

Or run: <code>crafters hint distributed-lock --stage expiry</code>
</details>
<!-- /crafters-stage-hint -->
