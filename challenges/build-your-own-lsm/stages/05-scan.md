# Stage 5: Range scan

Real LSM stores serve more than point lookups — they support range scans for
indexes, iterators, and prefix queries.

## Your task

Implement `scan`:

```
→ {"id": "1", "method": "scan", "params": {"start": "b", "end": "f"}}
← {"id": "1", "result": {"entries": [{"key": "c", "value": "3"}, {"key": "e", "value": "5"}]}}
```

**Semantics:**

- Half-open range `[start, end)` — `start` inclusive, `end` exclusive.
- Lexicographic (byte/string) ordering.
- Merge memtable and all SST files; newer writes win for duplicate keys.
- Deleted keys must not appear in results.
- Results sorted by key ascending.

## Tests

The tester scans the memtable alone, then flushes, adds more keys, and scans
across both layers. It also checks the half-open end boundary.

## Notes

- An empty range returns `"entries": []`.
- `scan` with `start >= end` returns no entries.
