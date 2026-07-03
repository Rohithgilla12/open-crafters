# Stage 9: The gauntlet

Nothing new on the wire — this stage stress-tests **concurrency** and
**throughput** across many lock names.

## Your task

Survive a mixed workload:

1. Many workers concurrently `acquire` on a small set of lock names — exactly
   **one** winner per lock per round; others get `LOCK_HELD`.
2. Winners `release`; repeat for several rounds.
3. A **throughput floor**: thousands of `try_acquire` / `release` cycles on one
   connection must finish within a generous time budget.

## What the tester checks

- No two workers hold the same lock in a round.
- Releases succeed for every winner.
- The post-contention throughput run completes within the time budget.

## Notes

- Grant + persist must be atomic per lock — check expiry, decide, issue token,
  and write state under one lock.
- Per-lock mutexes (or one mutex over the map) are fine; avoid a global
  convoy or fsync-per-acquire on the hot path.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Grant must be atomic: read state, check expiry, decide, write token — all under one mutex per engine (or per lock). JSON I/O stays outside the lock if you want, but check-and-grant cannot be split.

Or run: <code>crafters hint distributed-lock --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
