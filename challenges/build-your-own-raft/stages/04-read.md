# Stage 4: Read your writes

Once entries are **committed and applied**, any node may serve consistent reads.

## Your task

Implement **`get`**:

```
→ {"id": "1", "method": "get", "params": {"key": "foo"}}
← {"id": "1", "result": {"found": true, "value": "bar"}}
```

Apply all log entries up to `commit_index` before answering. Missing keys return
`{"found": false}`.

## Tests

After a committed `set` of `foo=bar`, the tester reads `foo` from **every node**
and expects the same value.

## Notes

- Reads may hit followers — they must still reflect committed state.
- Uncommitted leader writes must not appear in `get` results.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `get` on the leader reads from the applied state machine (committed entries only). Followers can return `NOT_LEADER` with the current leader hint, or you can serve linearizable reads from the leader after commit.

Or run: <code>crafters hint raft --stage read</code>
</details>
<!-- /crafters-stage-hint -->
