# Stage 2: Generate an ID

Snowflakes start with a single RPC that mints a 64-bit ID and returns it as a
decimal string.

## Your task

Implement `next_id` and `parse` (see `PROTOCOL.md` for the bit layout).

```json
→ {"id": "1", "method": "next_id", "params": {}}
← {"id": "1", "result": {"id": "1083765171494912"}}

→ {"id": "2", "method": "parse", "params": {"id": "1083765171494912"}}
← {"id": "2", "result": {"timestamp_ms": 1710000000123, "worker_id": 0, "sequence": 0}}
```

Also implement `configure` so tests can set `worker_id` later (default `0`).

## What the tester checks

- `next_id` returns a non-empty decimal string.
- `parse` decodes `timestamp_ms` (absolute wall clock), `worker_id`, and
  `sequence` in range.
- `configure` without `worker_id` → `INVALID_PARAMS`.

## Notes

- Use the epoch `1577836800000` from the protocol — tests decode your IDs with
  `parse`, not by guessing your encoding.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Pack `(timestamp - epoch) << 22 | worker_id << 12 | sequence` into a decimal string.

Or run: <code>crafters hint id-generator --stage next-id</code>
</details>
<!-- /crafters-stage-hint -->
