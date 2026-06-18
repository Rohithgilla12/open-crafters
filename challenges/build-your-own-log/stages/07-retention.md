# Stage 7: Retention keeps offsets absolute

A log can't grow forever, so old records are dropped — **retention**. The
subtle, crucial rule: dropping records must **not renumber** the survivors. If
record 7 exists and you delete records 0–4, record 7 is *still* record 7. This
is what lets a consumer hold a meaningful offset across time — and it's exactly
where naive implementations (that store an array and reindex) go wrong.

## Your task

Implement `truncate`: drop every record with offset `< before`. The topic's
`start_offset` rises to `before`; `end_offset` is unchanged; surviving records
keep their absolute offsets and values.

- `read` below `start_offset` → error code `OUT_OF_RANGE`.
- A consumer group's committed offset is **not** rewritten by retention — but if
  it now points below `start_offset`, that consumer fell behind, and its next
  `read` is `OUT_OF_RANGE` (real-world "consumer lag past retention").

## Tests

Append `r0..r5`; a slow group commits offset 1. `truncate(before=3)` → `stats`
is `{start:3, end:6}`; reading from 3 returns `r3, r4, r5` at offsets 3,4,5;
reading from 0 (and from the slow group's offset 1) is `OUT_OF_RANGE`; the slow
group's committed offset is still 1; a new append gets offset 6.

## Notes

- Track a per-topic base offset; storing records as `(base, values[])` makes
  offset = `base + index`, and truncation just trims the front and raises `base`
  — offsets stay absolute for free.
- Don't touch committed offsets on truncate. Lag is the consumer's problem to
  observe, not yours to hide.
