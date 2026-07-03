# Stage 8: Same history, same commands

Deterministic replay is Temporal's foundational invariant: if you replay the
same history twice, you must get **byte-identical commands**. Non-determinism
breaks workflow recovery — the server and worker would disagree on what
should happen next.

## Your task

Ensure **`replay` is pure**:

- No `time.now()`, `random()`, UUID generation, or external I/O when
  computing commands.
- The tester calls `replay` with the same history **20 times** and compares
  JSON output — any difference fails.

This applies to all workflow types you've implemented, not just `greet`.

## Notes

- In production SDKs, non-determinism is detected at runtime and fails the
  workflow task. Here, the tester checks you got it right upfront.
- If you need unique IDs in commands, derive them deterministically from
  history (event count, activity id, etc.) — or avoid needing them entirely
  for the workflows in this challenge.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** No `Math.random()`, no `Date.now()`, no reading files — replay is a pure function of history. Call `replay` twice with identical input; byte- identical command output.

Or run: <code>crafters hint workflow-sdk --stage determinism</code>
</details>
<!-- /crafters-stage-hint -->
