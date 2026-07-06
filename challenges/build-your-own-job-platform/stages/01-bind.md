# Stage 1: Boot the stack

The harness starts three **reference** services and your gateway. Implement `ping` on the gateway; the tester also pings scheduler, queue, and lock directly.

Read `SCHEDULER_ADDR`, `QUEUE_ADDR`, `LOCK_ADDR` from the environment.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Read `SCHEDULER_ADDR`, `QUEUE_ADDR`, `LOCK_ADDR` from the environment. Your gateway only needs `ping` for this stage; the harness pings the references directly.

Or run: <code>crafters hint job-platform --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
