# Stage 9: The gauntlet

Everything at once, under concurrency.

## Your task

Survive a stress test that hammers your server from **multiple concurrent
connections** across several filters:

- `add` items and immediately `contains` them on four independent filters,
- mixed traffic from eight concurrent connections,
- a sequential `delete_filter` check at the end (optional RPC, but the
  reference implements it).

Optionally implement `delete_filter` if you haven't already — it helps keep
the gauntlet tidy.

## What the tester checks

- Several connections run dozens of add/contains/delete operations in parallel.
- After the storm, every item that was successfully added must still return
  `maybe_present: true` on `contains`.

## Notes

- Protect shared state (the filter map and bit arrays) so concurrent writes
  don't corrupt bits.
- There is no crash/restart in this gauntlet — pure concurrency, in-memory.
- The gauntlet is the last stage — there is nothing after it but satisfaction.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Protect the filter map and bit arrays with a lock (or equivalent) so concurrent connections can add and probe safely. `delete_filter` is optional but handy for cleanup under churn.

Or run: <code>crafters hint bloom-filter --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
