# Stage 6: No false negatives

This stage stress-tests the contract that makes bloom filters useful in
production: **if you added it, contains must say true** — every time.

## Your task

Still no new methods. Make sure `add` and `contains` stay consistent under
volume.

## What the tester checks

- Creates a filter, adds **200 distinct items**.
- Calls `contains` on every one — all must return `maybe_present: true`.

## Notes

- A single wrong bit index or off-by-one in the hash loop will fail one of the
  200 probes.
- Idempotent adds (adding the same item twice) must not break anything.
