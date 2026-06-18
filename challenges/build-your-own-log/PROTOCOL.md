# Wire Protocol — Build your own log

This document specifies what your program must implement: an **append-only,
replayable log** — the abstraction underneath Kafka, Redpanda, and most
event-streaming systems. The tester grades you over TCP as producers and
consumers, and by `SIGKILL`ing your process to check that appended records,
retention, and committed offsets survive.

The on-disk format is **not** graded — persist however you like. The contract
is behavioral.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

Accept connections within 10 seconds; handle concurrent ones. The tester
`SIGKILL`s and restarts with the same `--data-dir`.

## Transport: newline-delimited JSON

One JSON object per line. Request `{"id","method","params"}`; response echoes
`id` with one of `result` or `error` (`{"code","message"}`). Unknown methods →
`UNKNOWN_METHOD`.

## The model

A **topic** is an ordered, append-only sequence of records. Each record gets a
monotonically increasing **offset**, starting at 0, **absolute and permanent**:
once a record is offset 7 it is offset 7 forever, even after earlier records are
deleted by retention. Reading is **non-destructive** — any consumer can read any
range any number of times. Consumers track *their own* position; the log doesn't
track it for them, except that you store each **consumer group**'s committed
offset on request.

This is the opposite of the message-queue challenge: nothing is consumed or
acked away; the log is the source of truth and consumers replay it.

## Methods

### `ping`
- `{}` → `{"message": "pong"}`

### `append`
- **params:** `{"topic": "<string>", "value": "<string>"}`
- **result:** `{"offset": <int>}` — the absolute offset assigned to this record
  (the topic's previous end). Topics are created on first append.
- Durable before acknowledged.

### `read`
- **params:** `{"topic": "<string>", "offset": <int>, "max": <int>}` —
  `max` optional (default 100), the most records to return.
- **result:** `{"records": [{"offset": <int>, "value": "<string>"}, ...],
  "next_offset": <int>}` — records starting at `offset` in order, and the offset
  to read from next.
  - At/after the end: `{"records": [], "next_offset": <end>}`.
  - Below the earliest retained offset: error code **`OUT_OF_RANGE`**.
  - Reads never modify anything.

### `commit_offset`
- **params:** `{"group": "<string>", "topic": "<string>", "offset": <int>}`
- **result:** `{}` — durably record this consumer group's position for the
  topic. Durable before acknowledged.

### `committed_offset`
- **params:** `{"group": "<string>", "topic": "<string>"}`
- **result:** `{"offset": <int>}` — the group's committed offset, or **0** if it
  has never committed (a fresh group starts at the beginning).

### `truncate`
- **params:** `{"topic": "<string>", "before": <int>}`
- **result:** `{}` — retention: drop every record with offset `< before`. The
  topic's earliest available offset rises to `before`; **offsets are not
  renumbered** and the end offset is unchanged. Durable before acknowledged.

### `stats`
- **params:** `{"topic": "<string>"}`
- **result:** `{"start_offset": <int>, "end_offset": <int>}` — the earliest
  retained offset and the next offset to be assigned (= total appended). A
  never-seen topic reports `{0, 0}`.

## Durability

`append`, `commit_offset`, and `truncate` are the durable events. After a
`SIGKILL` and restart:

- every appended record is present at its original absolute offset, and the next
  append continues from the correct offset,
- retention (the raised `start_offset`) is preserved,
- every committed group offset is preserved, so a consumer resumes exactly where
  it left off.

## A note on fsync

`SIGKILL` can't catch a missing `fsync`. Make each durable event durable before
you acknowledge it. That discipline is the lesson; the tester checks the rest.
