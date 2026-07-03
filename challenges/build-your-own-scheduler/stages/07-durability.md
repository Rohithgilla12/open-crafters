# Stage 7: Survive a crash

Fire times must be **absolute**, not "delay from boot" — the Temporal timer
insight applied to schedulers.

## Your task

Persist jobs to `--data-dir`. The tester will:

1. `schedule` with `delay_ms: 2000`.
2. `SIGKILL` your process after ~500ms.
3. Restart with the same `--data-dir`.
4. Expect the job to become pollable around the **original** fire time (~1.5s
   after restart), not 2s after restart.

Only acknowledge `schedule` after durable write (write-before-ack).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Persist the full job table to `--data-dir` after every schedule, complete, fail, or cancel. On boot reload and resume — due jobs pollable, leases expired jobs back to pending.

Or run: <code>crafters hint scheduler --stage durability</code>
</details>
<!-- /crafters-stage-hint -->
