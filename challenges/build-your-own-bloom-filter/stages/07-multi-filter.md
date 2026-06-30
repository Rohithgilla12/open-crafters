# Stage 7: Independent filters

Real services run many bloom filters side by side — one per shard, table, or
cache namespace.

## Your task

Ensure each `filter_id` has its own bit array. Adding to one filter must not
affect another.

## What the tester checks

- Creates filters `a` and `b`.
- Adds `"only-in-a"` to `a` and `"only-in-b"` to `b`.
- `contains("only-in-a")` on `a` → true; on `b` → false.
- `contains("only-in-b")` on `b` → true; on `a` → false.

## Notes

- A single global bit array keyed only by hash position would fail this stage.
- `FILTER_NOT_FOUND` still applies when the filter ID doesn't exist.
