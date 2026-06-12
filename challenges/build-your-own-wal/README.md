# Build your own WAL

Build the **write-ahead log** — the durability primitive underneath Postgres,
SQLite, Kafka, etcd, and every system that promises "if we said OK, it's on
disk".

The vehicle is a small key-value server, but the KV part is a warm-up. The
challenge is everything that happens to the bytes:

- a **CRC-framed, append-only record format**, specified to the byte — the
  tester parses the log you write, and writes logs for you to recover,
- **crash recovery** by log replay,
- **torn writes**: the tester truncates your log mid-record and expects you
  to keep every complete record and cleanly discard the tail,
- **corruption**: the tester flips a byte mid-log and expects recovery to
  stop at the first invalid record — checksums are not optional,
- **checkpointing**: snapshot the state, reset the log, get both the
  crash-ordering and the recovery precedence right,
- and a final **gauntlet**: 100 randomized ops, repeated SIGKILLs, and an
  offline audit that your on-disk files reconstruct to exactly the state you
  served.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [An in-memory key-value store](stages/02-kv.md) | `set` / `get` / `del` |
| 3 | [Survive a crash](stages/03-persist.md) | persistence, write-before-ack |
| 4 | [Write the log format](stages/04-format.md) | CRC32 record framing, byte-exact |
| 5 | [Recover from any log](stages/05-replay.md) | replay a tester-crafted log |
| 6 | [Torn writes](stages/06-torn-writes.md) | prefix recovery, tail truncation |
| 7 | [Detect corruption](stages/07-checksums.md) | stop at the first invalid record |
| 8 | [Snapshots and log truncation](stages/08-checkpoint.md) | `checkpoint`, crash ordering |
| 9 | [The gauntlet](stages/09-gauntlet.md) | randomized crash-recovery stress |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — unusually for open-crafters, it specifies a
**file format** as well as a wire protocol, and both are graded. Copy a
starter from [starters/](starters/), then:

```sh
./tester/tester --challenge build-your-own-wal \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-wal/python/](../../examples/solutions/build-your-own-wal/python/).
Resist it until you've fought stage 6 — recovering a torn log correctly is
the heart of this challenge.
