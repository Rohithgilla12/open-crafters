# Hints — distributed KV

## Partitioning

`partition = hash(key) % num_partitions` or **consistent hash ring** with vnodes — your **hash ring** challenge.

Each partition has a **replica set** of 3 nodes (leader + followers).

## Replication

**Raft per partition** (etcd style) OR **leaderless quorum** (Dynamo style: W/R/N tuning).

Pick one and stick to it in the interview.

## Strong read

Route to leader, or quorum read (R + W > N) — explain trade-off.

## Rebalancing

On add node: ring moves some partitions → copy data → switch traffic. Use **minimal movement** from consistent hashing.

## Storage engine

Per-node **LSM-tree** (RocksDB) for write-heavy KV — your **LSM** challenge. Memtable + SSTables, compaction.

## open-crafters tie-in

- **Raft** — leader election + replicated log per shard
- **Hash ring** — key placement + rebalancing
- **LSM** — on-disk layout per node
