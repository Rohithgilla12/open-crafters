# Stage 8: Parallel sends

Several clients can send to different conversations concurrently without corrupting history.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Each TCP connection to your gateway can handle one client; child RPCs are stateless.

Or run: <code>crafters hint chat-service --stage concurrent</code>
</details>
<!-- /crafters-stage-hint -->
