# Stage 3: Replicate a write

A Raft leader must **replicate** each client write to a quorum before acknowledging
it.

## Your task

Implement **`set`** on the leader:

```
→ {"id": "1", "method": "set", "params": {"key": "replicate", "value": "ok"}}
← {"id": "1", "result": {"index": 1}}
```

Rules:

- Only the **leader** accepts writes. Followers must reject with `NOT_LEADER`
  (include `"leader_id"` when known).
- Do not acknowledge until a **quorum** has replicated the entry and
  `commit_index` advances.
- Non-leaders must not acknowledge client writes.

## Tests

The tester writes via the leader, then waits until **every running node** reports
`commit_index >= 1`.

## Notes

- Log entries carry `index`, `term`, `key`, and `value` (see PROTOCOL.md).
- This is the same write-before-ack discipline as [Build your own WAL](../../build-your-own-wal/) — but replicated.
