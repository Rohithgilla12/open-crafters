# Stage 2: Send a message

Implement `send_message` — mint a `message_id` via id-generator `next_id`, append a JSON envelope to the log (`topic` = `conversation_id`), enqueue on queue `delivery`, return `message_id` and log `offset`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Envelope fields: `message_id`, `conversation_id`, `sender`, `body`.

Or run: <code>crafters hint chat-service --stage send</code>
</details>
<!-- /crafters-stage-hint -->
