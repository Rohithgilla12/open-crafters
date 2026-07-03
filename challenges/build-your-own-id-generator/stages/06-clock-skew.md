# Stage 6: Dense millisecond allocation

A single `batch` call should mint many IDs inside one millisecond — the
sequence counter increments without advancing time.

## Your task

When `batch` allocates multiple IDs in one request, keep the same
`timestamp_ms` until sequence would exceed `4095`, then wait for the next
millisecond (see `CLOCK_BACKWARDS` in the protocol).

## What the tester checks

- `batch` with `count: 512` — all IDs share one `timestamp_ms`, sequences
  `0` through `511`.
- The following `next_id` succeeds (sequence `512` in the same ms, or a new ms).

## Notes

- Implement `batch` as a loop over the same allocator as `next_id` — do not
  call `time.Now()` once per ID if you can help it; reuse the current
  millisecond bucket.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** For `batch`, capture one `now_ms()` bucket and mint many IDs before re-reading the clock.

Or run: <code>crafters hint id-generator --stage clock-skew</code>
</details>
<!-- /crafters-stage-hint -->
