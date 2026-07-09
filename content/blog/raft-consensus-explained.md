---
title: Raft consensus without the mystique
description: Leader election, log replication, and partition safety — how a 3-node Raft cluster agrees under crashes and network splits.
date: 2026-07-07
tags: consensus, raft, distributed
related: build-your-own-raft, design-distributed-kv, design-coordination-service
---

Raft is not “magic distributed agreement.” It is a protocol with three jobs:

1. **Elect** a leader
2. **Replicate** an ordered log
3. **Stay safe** when the network partitions

## Why three nodes

With three voters, a majority is two. One node can disappear and the cluster still makes progress. Two failures freeze writes — by design — rather than risk split-brain.

## What we grade

[Build your own Raft](/challenges/build-your-own-raft) runs a live 3-node cluster. Stages cover election, replication, crash recovery, and partitions. The harness injects failures; your nodes must refuse unsafe commits.

## Where it shows up

Raft underpins coordination services and per-shard leadership in distributed KV designs. See [Design a distributed KV store](/design/design-distributed-kv) and [Design a coordination service](/design/design-coordination-service).

```sh
crafters start raft
```
