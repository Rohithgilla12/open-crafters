# Stage 8: Parallel calls

Two concurrent `call` RPCs through your harness on the **same** spawned child
must both return `{"message": "pong"}`.

Your harness must handle concurrent requests on one connection (or multiple
connections) safely.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Mutex around shared maps if needed. Two goroutines call `call` on the same spawned `addr` concurrently — both must succeed.

Or run: <code>crafters hint harness --stage concurrent</code>
</details>
<!-- /crafters-stage-hint -->
