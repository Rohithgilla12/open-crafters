# Stage 2: Create a filter

Before you can add items, clients need a named bloom filter with a bit array
of size `m` and `k` hash functions.

## Your task

Implement `create`:

```json
→ {"id":"1","method":"create","params":{"filter_id":"users","m":1024,"k":3}}
← {"id":"1","result":{}}
```

- `m` — number of bits, must be **≥ 8**.
- `k` — number of hash functions, must be **≥ 1**.
- Duplicate `filter_id` → error `FILTER_EXISTS`.
- Invalid or missing params → error `INVALID_PARAMS`.

## What the tester checks

- A valid `create` succeeds with an empty `{}` result.
- Creating the same `filter_id` twice returns `FILTER_EXISTS`.
- `m=7`, `k=0`, or missing fields return `INVALID_PARAMS`.

## Notes

- Allocate the bit array now; you'll hash into it in the next stage.
- See [PROTOCOL.md](../PROTOCOL.md) for the exact hash specification.
