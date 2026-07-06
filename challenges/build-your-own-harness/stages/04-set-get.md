# Stage 4: Set and get via proxy

Use `spawn` once, then two `call` RPCs on the same child:

1. `set` `{"key": "color", "value": "blue"}`
2. `get` `{"key": "color"}` → `{"hit": true, "value": "blue"}`

State must persist within one spawned process.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Reuse `spawn` once, then two `call` RPCs on the same `addr`: `set` then `get`. The toy returns `hit` and `value` on a hit.

Or run: <code>crafters hint harness --stage set-get</code>
</details>
<!-- /crafters-stage-hint -->
