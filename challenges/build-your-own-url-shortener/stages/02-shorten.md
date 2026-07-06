# Stage 2: Mint a short code

`shorten` with `{"url": "..."}` → `{"code": "..."}`.

Orchestrate: `next_id` → bloom `add` → object store `put` at `links/<code>`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `next_id` → bloom `add` on filter `codes` → object store `put` at `links/<code>`.

Or run: <code>crafters hint url-shortener --stage shorten</code>
</details>
<!-- /crafters-stage-hint -->
