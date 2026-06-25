# Build your own LSM-tree

Build the **log-structured merge-tree** — the storage engine behind RocksDB,
LevelDB, Cassandra, and every system that turns random writes into sequential
I/O.

The vehicle is a small key-value server, but the substance is the LSM
lifecycle:

- an in-memory **memtable** for fast writes,
- **SSTables** on disk in a byte-exact format the tester parses,
- **flush** to make memtable data durable,
- **range scans** across memtable + SST layers,
- **compaction** to merge overlapping files,
- **tombstones** for deletes that survive restarts,
- and a final **gauntlet**: randomized ops, repeated SIGKILLs, and an offline
  audit that your SST files reconstruct to exactly the state you served.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [An in-memory key-value store](stages/02-put-get.md) | `put` / `get` / `del` |
| 3 | [Flush to SSTable](stages/03-flush.md) | `flush`, byte-exact SST1 format |
| 4 | [Recover after restart](stages/04-restart.md) | load SST files on startup |
| 5 | [Range scan](stages/05-scan.md) | `scan` across memtable + SST |
| 6 | [Compact SSTables](stages/06-compaction.md) | merge files, latest value wins |
| 7 | [Tombstones](stages/07-delete.md) | `value_len=0` hides keys |
| 8 | [Multi-file recovery](stages/08-durability.md) | multiple SST files, compact |
| 9 | [The gauntlet](stages/09-gauntlet.md) | randomized crash-recovery stress |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — it specifies a **file format** as well as a
wire protocol, and both are graded. Copy a starter from [starters/](starters/),
then:

```sh
./crafters grade --challenge build-your-own-lsm \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-lsm/python/](../../examples/solutions/build-your-own-lsm/python/).
Resist it until you've fought stage 6 — merging SSTables with overlapping
keys correctly is the heart of this challenge.
