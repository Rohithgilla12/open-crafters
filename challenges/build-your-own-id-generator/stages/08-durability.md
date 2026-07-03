# Stage 8: Survive a crash

An ID generator that resets its counter on restart will **reuse** IDs — two
different records could get the same primary key.

## Your task

Persist `last_timestamp_ms`, `last_sequence`, and `worker_id` under
`--data-dir`. After `SIGKILL` + restart, the next IDs must be **strictly
greater** than every ID issued before the crash.

## What the tester checks

- Issue 200 IDs, kill the process, restart with the same `--data-dir`.
- The next 50 IDs are all greater than the pre-crash maximum and never repeat
  an earlier value.

## Notes

- Same durability discipline as the WAL and distributed-lock challenges: write,
  `fsync` if you like, atomic rename — but **correctness** is what we grade.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Persist `last_timestamp_ms` and `last_sequence` after each allocation — atomic rename is enough.

Or run: <code>crafters hint id-generator --stage durability</code>
</details>
<!-- /crafters-stage-hint -->
