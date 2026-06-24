# Stage 2: Leader election

Raft needs a **leader** to coordinate replication. With three healthy nodes, exactly
one leader must emerge within a few seconds.

## Your task

Implement **`get_status`** and the Raft machinery to elect a leader:

```
→ {"id": "1", "method": "get_status", "params": {}}
← {"id": "1", "result": {
     "node_id": "1",
     "role": "leader",
     "term": 2,
     "leader_id": "1",
     "commit_index": 0,
     "last_applied": 0
   }}
```

- `role` is `leader`, `follower`, or `candidate`.
- `leader_id` is `"0"` when unknown.
- Use `--peers` to reach other nodes for **`request_vote`** and **`append_entries`**
  (heartbeats). See [PROTOCOL.md](../PROTOCOL.md).

## Tests

The tester polls all three nodes until exactly one reports `role: "leader"` and
every node agrees on the same `leader_id`.

## Notes

- Election timeouts ≥ **300ms**; heartbeats ≤ **150ms** (CI stability).
- At most one leader per term among reachable nodes.
