# Stage 9: The gauntlet

Mixed checks:

- Four `run_case` assertions on the toy (ping, get miss, set, ping).
- `spawn` + multiple `call` set/get on three keys in **one** child.

Exercises assertion logic and stateful proxying together.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Run several `run_case` assertions (ping, get miss, set), then `spawn` once and exercise three keys via `call` set/get in one child.

Or run: <code>crafters hint harness --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
