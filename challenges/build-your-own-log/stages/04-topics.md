# Stage 4: Independent topics

Real systems carry many streams at once — `orders`, `clicks`, `audit` — and they
must not bleed into each other. Each topic is its own independent log with its
own offset space.

## Your task

Key everything by topic. Each topic's offsets start at 0 and advance
independently. Add `stats`:

```
→ {"id":"1","method":"stats","params":{"topic":"orders"}}
← {"id":"1","result":{"start_offset":0,"end_offset":3}}
```

`start_offset` is the earliest available offset (0 until retention), `end_offset`
is the next offset to be assigned (= number appended). A never-seen topic
reports `{0, 0}`.

## Tests

Append different counts to two topics; offsets are independent; reads are
isolated; `stats` reports each topic's start/end, and an unknown topic reports
zeroes.

## Notes

- If your state is a map from topic name to its log, you're basically done.
