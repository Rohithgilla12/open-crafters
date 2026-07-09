# Stage 7: Two conversations

Messages in `room-a` must not appear when reading `room-b` — each `conversation_id` is a separate log topic.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Use `conversation_id` as the log topic on every append and read.

Or run: <code>crafters hint chat-service --stage two-chats</code>
</details>
<!-- /crafters-stage-hint -->
