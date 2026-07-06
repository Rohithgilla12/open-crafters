# Stage 8: Parallel shortens

12 concurrent `shorten` calls must produce **unique** codes.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Mutex around shared client state if you cache connections; child services handle their own concurrency.

Or run: <code>crafters hint url-shortener --stage concurrent</code>
</details>
<!-- /crafters-stage-hint -->
