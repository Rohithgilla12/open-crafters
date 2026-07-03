# Stage 9: The gauntlet

Everything at once, with crashes in the middle.

## Your task

Pass a stress test that interleaves `put`, `get`, `head`, `delete`, `list`, and
multipart uploads across several rounds. Between rounds the tester `SIGKILL`s
your process and restarts it.

## Tests

- Random mix of operations over multiple keys and prefixes.
- After each restart, every object that was successfully written (via `put` or
  completed multipart) must still be present with the correct body.
- Deleted keys must stay deleted.
- `head` sizes must match stored bodies.
- `list` under `g/` must reflect the surviving key set.

## Notes

- If you have been persisting after every mutation since stage 8, this stage
  is mostly about getting multipart and delete semantics right under crash
  pressure.
- The gauntlet is the last stage — there is nothing after it but satisfaction.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Compose everything: put, get, head, delete, list, multipart, persist after every change. A crash mid-gauntlet must not lose acknowledged writes or resurrect deleted keys.

Or run: <code>crafters hint object-store --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
