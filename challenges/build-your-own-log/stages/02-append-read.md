# Stage 2: Append and read by offset

A log is the simplest durable data structure there is: you only ever add to the
end, and every record gets a number — its **offset** — that never changes. That
single idea (an ordered, addressable, append-only sequence) is what lets many
independent consumers read the same stream at their own pace. Build the core
here.

## Your task

Implement `append` and `read`.

```
→ {"id":"1","method":"append","params":{"topic":"t","value":"a"}}
← {"id":"1","result":{"offset":0}}

→ {"id":"2","method":"read","params":{"topic":"t","offset":0}}
← {"id":"2","result":{"records":[{"offset":0,"value":"a"}],"next_offset":1}}
```

- **`append`** adds a record and returns its offset — 0-based, increasing by one
  each time, per topic.
- **`read`** returns records starting at `offset`, in order, plus `next_offset`
  (where to read next). Reading at or past the end returns `{"records":[],
  "next_offset":<end>}`. Reading is **non-destructive** — the same read returns
  the same records every time.

## Tests

Append `a`, `b`, `c` → offsets 0, 1, 2. Read from 0 returns all three with
`next_offset` 3; from 1 returns `b`, `c`; re-reading from 0 returns the same;
reading at the end returns empty.

## Notes

- A list per topic, where index = offset, is all you need for now.
- "Reads don't consume" is the whole personality of a log — don't pop or delete
  on read.
