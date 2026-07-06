# Stage 3: Proxy a call

Implement `call` — forward one NDJSON RPC to a child address.

```
→ {"id": "1", "method": "call", "params": {"addr": "127.0.0.1:54321", "method": "ping", "params": {}}}
← {"id": "1", "result": {"message": "pong"}}
```

The tester spawns the toy, then calls `ping` through your harness proxy.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Open a one-shot TCP client to `addr`, send one NDJSON request with the forwarded `method` and `params`, read one line, return the child's `result` or propagate its `error` code.

Or run: <code>crafters hint harness --stage call</code>
</details>
<!-- /crafters-stage-hint -->
