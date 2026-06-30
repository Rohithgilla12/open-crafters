# Stage 8: K hash functions

The difference between a bloom filter and a single-hash bitmap is **k**
independent-looking positions per item, derived from double hashing.

## Your task

Implement the full scheme from [PROTOCOL.md](../PROTOCOL.md):

```
h1 = fnv1a64(item_bytes)
h2 = fnv1a64(item_bytes + byte 0x01)
position_i = (h1 + i * h2) % m   for i in 0..k-1
```

Both `add` and `contains` must use all k positions.

## What the tester checks

1. **Single-hash cheat detection:** With `m=64`, `k=3`, only `"alpha"` added,
   a crafted `"beta"` (where `h1(beta) % m == h1(alpha) % m` but not all k
   bits match) must return `maybe_present: false`. An implementation that only
   checks `h1 % m` would falsely return true.

2. **k=1 vs k=2:** Filters with different `k` values behave differently for
   a near-collision item — `k=2` requires two bits set, `k=1` requires one.

## Notes

- `% m` applies to the **64-bit hash value**, then you use that as a bit index
  in `0 .. m-1`.
- This stage is why the hash spec is pinned exactly — the tester computes
  expected positions with the same algorithm.
