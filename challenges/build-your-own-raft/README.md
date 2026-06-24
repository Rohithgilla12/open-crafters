# Build your own Raft

Build a **3-node Raft cluster** that replicates a key-value state machine: leader
election, log replication, crash recovery, and partition safety — graded over a live
multi-node cluster with network partitions.

If you completed [Build your own WAL](../build-your-own-wal/), you already know
**write-before-ack**. This challenge applies that discipline to a **replicated log**:
the leader must not acknowledge a client write until a quorum has persisted it.

Your cluster will:

- boot **three nodes** from the same program binary,
- **elect a leader** and replicate writes through Raft,
- serve **linearizable reads** from committed state,
- survive **follower and leader crashes**,
- **persist** across total cluster failure, and
- stay safe under **network partitions**.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the cluster](stages/01-bind.md) | TCP + `ping` with `node_id` |
| 2 | [Leader election](stages/02-leader.md) | `get_status`, Raft elections |
| 3 | [Replicate a write](stages/03-replicate.md) | `set` through the log |
| 4 | [Read your writes](stages/04-read.md) | `get` from applied state |
| 5 | [Survive a follower crash](stages/05-follower-crash.md) | commit with one node down |
| 6 | [Survive a leader crash](stages/06-leader-crash.md) | re-elect, preserve log |
| 7 | [Survive a full crash](stages/07-durability.md) | persist to `--data-dir` |
| 8 | [Partition safety](stages/08-partition.md) | no quorum → no commit |
| 9 | [The gauntlet](stages/09-gauntlet.md) | writes, crash, restart |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — it's the complete contract. Copy a starter from
[starters/](starters/) (Python, Go, and TypeScript available), then:

```sh
crafters start raft
crafters test
```

**Prerequisite:** [Build your own WAL](../build-your-own-wal/) teaches the
durability mindset Raft builds on.

Stuck? Reference solutions live in
[examples/solutions/build-your-own-raft/](../../examples/solutions/build-your-own-raft/).
