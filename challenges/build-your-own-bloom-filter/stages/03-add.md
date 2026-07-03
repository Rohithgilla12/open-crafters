# Stage 3: Add an item

Membership starts with insertion: hash the item to k bit positions and set
those bits.

## Your task

Implement `add`:

```json
→ {"id":"1","method":"add","params":{"filter_id":"tags","item":"golang"}}
← {"id":"1","result":{}}
```

Use the FNV-1a double-hash scheme from [PROTOCOL.md](../PROTOCOL.md) to
compute k positions, then set each bit to 1.

- Unknown `filter_id` → error `FILTER_NOT_FOUND`.

## What the tester checks

- `add` on an existing filter succeeds.
- `add` on a missing filter returns `FILTER_NOT_FOUND`.

## Notes

- Items are UTF-8 strings — hash the raw bytes, not a language-specific string
  representation.
- Adding the same item twice is fine (bits stay set).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Hash the item's UTF-8 bytes with the PROTOCOL's FNV-1a scheme to get k positions, then set those bits to 1. Unknown filter → `FILTER_NOT_FOUND`.

Or run: <code>crafters hint bloom-filter --stage add</code>
</details>
<!-- /crafters-stage-hint -->
