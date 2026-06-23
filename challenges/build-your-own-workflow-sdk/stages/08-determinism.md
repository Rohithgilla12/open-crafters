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
