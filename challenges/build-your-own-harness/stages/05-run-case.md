# Stage 5: Run one case

Implement `run_case` — spawn, call one method, assert subset equality.

```
→ {"id": "1", "method": "run_case", "params": {
    "program": "/path/to/toy",
    "method": "ping",
    "params": {},
    "expect": {"message": "pong"}
  }}
← {"id": "1", "result": {}}
```

Return `{}` on success. Mismatch → `CASE_FAILED`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `run_case` = spawn + call + subset check. Every key in `expect` must match the child's `result`; extra result keys are fine. Return `{}` on success, `CASE_FAILED` on mismatch.

Or run: <code>crafters hint harness --stage run-case</code>
</details>
<!-- /crafters-stage-hint -->
