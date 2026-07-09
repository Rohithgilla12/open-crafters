# Stage 6: Message order

Multiple `send_message` calls to the same conversation must appear in append order when reading from offset 0.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** The log preserves per-topic ordering; don't reorder in the gateway.

Or run: <code>crafters hint chat-service --stage ordering</code>
</details>
<!-- /crafters-stage-hint -->
