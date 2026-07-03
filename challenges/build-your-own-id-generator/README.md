# Build your own ID generator

Generate **snowflake-style** 64-bit identifiers: time-ordered, unique across
workers, dense within a millisecond, and **durable across crashes**.

You will build a TCP service that speaks newline-delimited JSON and implements
`next_id`, `batch`, `configure`, and `parse`. The tester grades you entirely
over the wire — including `SIGKILL` to verify you never reuse an ID.

## Stages

| # | Slug | What you build |
|---|------|----------------|
| 1 | `bind` | TCP + NDJSON + `ping` |
| 2 | `next-id` | `next_id` + `parse` |
| 3 | `sortable` | Strictly increasing IDs over time |
| 4 | `worker-id` | `configure` worker bits |
| 5 | `sequence` | 5000 IDs/ms without collision |
| 6 | `clock-skew` | Sequence rollover at 4095 |
| 7 | `batch` | `batch` up to 1024 ascending IDs |
| 8 | `durability` | Survive `SIGKILL` without reuse |
| 9 | `gauntlet` | Concurrent uniqueness + throughput |

Read `PROTOCOL.md` for the exact bit layout and RPC contracts.

## Quick start

```bash
crafters start id-generator
cd build-your-own-id-generator
crafters test
```

The Go/Python/TypeScript starters pass stage 1. Implement snowflake allocation
for stage 2 onward.

## Why this matters

Twitter's Snowflake, Sonyflake, and every sharded database need **ordered,
unique, roughly-time-sorted** IDs without a central database sequence. The
pattern shows up in job schedulers (this catalog's scheduler challenge), event
logs, and primary keys at scale.
