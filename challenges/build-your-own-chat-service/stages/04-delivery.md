# Stage 4: Fan-out queue

Implement `poll_delivery` — non-blocking `receive` on queue `delivery`. Parse the envelope and include `receipt` from the queue message.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Return `{"message": null}` when the queue is empty.

Or run: <code>crafters hint chat-service --stage delivery</code>
</details>
<!-- /crafters-stage-hint -->
