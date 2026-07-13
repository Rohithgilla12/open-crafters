---
title: LSM-trees turn random writes into sequential I/O
description: Memtables, byte-exact SSTables, range scans, compaction, and tombstones — how RocksDB-style engines stay durable under crash.
date: 2026-07-12
tags: lsm, storage, sstable
related: build-your-own-lsm, design-distributed-kv
---

Databases that absorb heavy write traffic rarely update pages in place. They buffer in a **memtable**, flush sorted **SSTables**, and merge in the background. That lifecycle is the log-structured merge-tree.

## The contract

- Writes hit an in-memory memtable first, then flush to immutable SST files
- **Latest value wins** across overlapping files after compaction
- **Tombstones** hide deleted keys until compaction can drop them
- After a crash, reopening the SST set reconstructs exactly what you acknowledged

## What we grade

[Build your own LSM-tree](/challenges/build-your-own-lsm) grades both the wire API and a **byte-exact SST format** the tester parses offline. SIGKILL gauntlets check that flushed state survives and matches what you served.

## Start here

```sh
crafters start lsm
```

On the [durability roadmap](/roadmaps/durability) this sits between the append log and MVCC.
