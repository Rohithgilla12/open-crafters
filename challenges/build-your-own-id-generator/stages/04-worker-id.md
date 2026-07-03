# Stage 4: Worker partitioning

Multiple processes (or logical workers) embed their identity in the ID.

## Your task

`configure` sets the 10-bit `worker_id` (`0`–`1023`). `parse` must reflect the
configured value in every subsequent ID.

```json
→ {"id": "1", "method": "configure", "params": {"worker_id": 42}}
← {"id": "1", "result": {}}
```

## What the tester checks

- After `configure` with `42`, `parse` reports `worker_id: 42`.
- Reconfigure to `1000` — new IDs reflect the new worker.
- Out-of-range `worker_id` → `INVALID_PARAMS`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Store `worker_id` from `configure` and OR it into every composed ID.

Or run: <code>crafters hint id-generator --stage worker-id</code>
</details>
<!-- /crafters-stage-hint -->
