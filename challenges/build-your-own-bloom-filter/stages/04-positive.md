# Stage 4: Positive lookup

Time to probe membership. The golden rule of bloom filters: **no false
negatives**.

## Your task

Implement `contains`:

```json
→ {"id":"1","method":"contains","params":{"filter_id":"emails","item":"alice@example.com"}}
← {"id":"1","result":{"maybe_present":true}}
```

An item you have `add`ed must return `maybe_present: true`. Check that **all**
k bit positions are set.

- Unknown `filter_id` → error `FILTER_NOT_FOUND`.

## What the tester checks

- After `add`, `contains` on the same item returns `maybe_present: true`.

## Notes

- The field is `maybe_present`, not `present` — bloom filters can false-positive
  but never false-negative on items that were added.
