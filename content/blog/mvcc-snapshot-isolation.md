---
title: Snapshot isolation is multi-version reads
description: Frozen snapshots, buffered writes, and first-committer-wins conflicts — how transactional KV stores stay consistent under concurrency.
date: 2026-07-13
tags: mvcc, transactions, isolation
related: build-your-own-mvcc, design-payment-ledger
---

A transactional store does not hand every reader the latest write. Under **snapshot isolation**, a transaction freezes a consistent view at begin time, buffers its own writes, and only publishes them if no conflicting commit landed first.

## The contract

- Reads see a **snapshot** of committed data as of `begin`, plus the txn’s own writes
- Concurrent writers that touch the same key: **first committer wins**; the loser aborts
- Uncommitted work is invisible to others and disappears on rollback or crash
- Only **committed** transactions survive `SIGKILL`

## What we grade

[Build your own MVCC](/challenges/build-your-own-mvcc) runs overlapping clients, checks snapshot reads, atomic multi-key commits, conflict detection, deletes, and crash durability. On-disk layout is yours — observable isolation is not.

## Start here

```sh
crafters start mvcc
```

See it in an interview framing: [Design a payment ledger](/design/design-payment-ledger).
