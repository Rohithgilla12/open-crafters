# Stage 5: Negative lookup

Bloom filters trade exactness for space: items you never added *might* appear
present (false positives), but on a **sparse** filter they usually won't.

## Your task

Keep working `contains` — no new methods. When the filter is mostly empty,
never-added items should return `maybe_present: false`.

## What the tester checks

- Creates a filter with `m=1024`, `k=3`.
- Adds at most two items (keeping the filter sparse).
- Probes **ten unique items that were never added** — all must return
  `maybe_present: false`.

## Notes

- False positives are expected in theory once the filter fills up; this stage
  only checks the easy case where the filter is nearly empty.
- If you get false positives here, double-check your hash positions — a bug
  that sets too many bits looks like a full filter.
