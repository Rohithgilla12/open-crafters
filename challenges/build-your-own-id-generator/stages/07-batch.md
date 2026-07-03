# Stage 7: Batch allocate

Clients often need a run of IDs — one RPC should mint many, in order.

## Your task

Implement `batch`:

```json
→ {"id": "1", "method": "batch", "params": {"count": 100}}
← {"id": "1", "result": {"ids": ["...", "..."]}}
```

Returned IDs must be **unique** and **strictly increasing** (as integers).

## What the tester checks

- `batch` with `count: 100` — 100 ascending unique IDs.
- `count: 0` → `INVALID_PARAMS`; `count: 1025` → `BATCH_TOO_LARGE`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Loop the same allocator as `next_id`; cap at 1024 per call.

Or run: <code>crafters hint id-generator --stage batch</code>
</details>
<!-- /crafters-stage-hint -->
