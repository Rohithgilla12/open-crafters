---
title: Why every database starts with a write-ahead log
description: Crash safety is write-before-ack. How WALs frame records, detect torn writes, and why open-crafters grades your log byte-for-byte.
date: 2026-07-09
tags: durability, wal, storage
related: build-your-own-wal, design-blob-storage
---

Production storage systems share one discipline: **acknowledge work only after it is durable**. A write-ahead log (WAL) is the smallest primitive that enforces that rule.

## The promise

Before you update an in-memory table or reply `OK` to a client, you append a framed record to an append-only file and `fsync`. If the process dies mid-update, recovery replays the log and reconstructs state.

That is the entire contract. Everything else — CRC framing, torn-write detection, checkpoints — exists to keep that contract honest under power loss and partial sector writes.

## What graders care about

In [Build your own WAL](/challenges/build-your-own-wal), the harness does not trust your API alone. It:

1. Writes through your protocol
2. Kills the process
3. Parses your on-disk log
4. Checks that recovered state matches what you acknowledged

If you ack before the bytes hit stable storage, you fail. That is the point.

## Start here

```sh
crafters start wal
```

Or open the [durability roadmap](/roadmaps/durability) and work WAL → queue → log → LSM → MVCC → object store.
