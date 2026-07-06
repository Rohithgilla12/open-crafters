# Stage 6: Run a suite

Run **three** `run_case` assertions in sequence:

1. `ping` → expect `{"message": "pong"}`
2. `get` `{"key": "missing"}` → expect `{"hit": false}`
3. `set` `{"key": "suite", "value": "ok"}` → expect `{}`

Each `run_case` spawns a **fresh** child.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Each `run_case` spawns a **fresh** child — cases are isolated. Run three assertions: ping, get miss, set success.

Or run: <code>crafters hint harness --stage run-suite</code>
</details>
<!-- /crafters-stage-hint -->
