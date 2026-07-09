# Stage 1: Boot the stack

Implement `ping` returning `{"message": "pong"}`. The harness checks that your gateway and all three reference services respond.

On first real use, configure the id-generator (`configure` with `worker_id`).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `ping` is enough for bind; read `IDGEN_ADDR`, `LOG_ADDR`, and `QUEUE_ADDR` from the environment for later stages.

Or run: <code>crafters hint chat-service --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
