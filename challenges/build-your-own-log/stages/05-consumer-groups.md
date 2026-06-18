# Stage 5: Consumer group offsets

Here's the trick a log plays that a queue can't: it doesn't track who has read
what — consumers track *themselves*. A consumer reads from offset N, processes
the records, and tells the log "I've now committed through offset M." If it
crashes, it asks the log "where was I?" and resumes. Many independent consumer
**groups** can replay the same topic at totally different positions.

## Your task

Implement `commit_offset` and `committed_offset`, keyed by `(group, topic)`.

```
→ {"method":"commit_offset","params":{"group":"g1","topic":"t","offset":3}}
← {"result":{}}
→ {"method":"committed_offset","params":{"group":"g1","topic":"t"}}
← {"result":{"offset":3}}
```

A group that has never committed reports offset **0** (start from the
beginning). Groups are independent of one another.

## Tests

- A fresh group reports 0; after committing 3 it reports 3; a second group is
  unaffected (still 0).
- The "consume" loop: read from your committed offset, then commit `next_offset`
  — and reading never advanced anything on its own.

## Notes

- The committed offset is just a number you store and hand back. The log neither
  validates it against the data nor advances it for the consumer — that's the
  consumer's job, and the source of a log's scalability.
