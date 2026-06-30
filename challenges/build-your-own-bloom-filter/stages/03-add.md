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
