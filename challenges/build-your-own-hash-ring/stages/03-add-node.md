# Stage 3: Add a node

Add physical nodes to a ring and route keys to them.

## Your task

Implement `add_node`:

```json
→ {"id":"1","method":"add_node","params":{"ring_id":"solo","node_id":"only-one"}}
← {"id":"1","result":{}}
```

Also implement `lookup`:

```json
→ {"id":"2","method":"lookup","params":{"ring_id":"solo","key":"alpha"}}
← {"id":"2","result":{"node_id":"only-one"}}
```

- Unknown ring → `RING_NOT_FOUND`.
- Duplicate `node_id` → `NODE_EXISTS`.
- `lookup` on a ring with no nodes → `NO_NODES`.

## What the tester checks

- One node on a ring: every lookup returns that node.
- Duplicate add and missing ring errors.

## Notes

- Implement `lookup` with the PROTOCOL hash walk in this stage or the next.
