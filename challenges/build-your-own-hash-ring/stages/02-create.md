# Stage 2: Create a ring

Before you can place keys, clients need a named ring with a virtual-node count
(`replicas` per physical node).

## Your task

Implement `create_ring`:

```json
→ {"id":"1","method":"create_ring","params":{"ring_id":"cache","replicas":3}}
← {"id":"1","result":{}}
```

- `replicas` — virtual nodes per physical node, must be **≥ 1**.
- Duplicate `ring_id` → error `RING_EXISTS`.
- Invalid or missing params → error `INVALID_PARAMS`.

## What the tester checks

- A valid `create_ring` succeeds with an empty `{}` result.
- Creating the same `ring_id` twice returns `RING_EXISTS`.
- `replicas=0`, negative replicas, or missing fields return `INVALID_PARAMS`.

## Notes

- Store `replicas` on the ring; vnodes are derived at lookup time from
  `node_id + "#" + replica index per [PROTOCOL.md](../PROTOCOL.md).
