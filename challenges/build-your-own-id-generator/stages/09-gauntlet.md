# Stage 9: The gauntlet

Nothing new on the wire — stress **concurrency** and **throughput**.

## Your task

Survive:

1. **30 concurrent connections**, each issuing **150** `next_id` calls — every
   ID globally unique.
2. A single-connection **hot path**: **3000** mixed `next_id` / `batch` calls
   within **8 seconds**.

## What the tester checks

- No duplicate IDs under concurrent load.
- Throughput floor on the hot path.

## Notes

- Protect `(last_timestamp_ms, last_sequence)` with a mutex — the gauntlet will
  find races if you don't.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** One mutex, one state file, no per-connection counters.

Or run: <code>crafters hint id-generator --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
