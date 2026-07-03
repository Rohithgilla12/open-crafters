# Hints — coordination service

Open only if you're stuck.

## Consensus core

3 or 5 nodes run **Raft** — your **raft** challenge. One leader; all writes go through leader's log.

```
Client PUT ──▶ Leader ──▶ append Raft log entry
                │
                ├── replicate to followers
                ├── commit on majority
                └── apply to state machine
```

Never split-brain: need quorum for leader election and commit.

## Data model

Hierarchical keys: `/services/payment/leader`, `/config/feature-flags`

Each key has `{value, version, lease_id?, create_revision, mod_revision}`.

Revisions are cluster-wide monotonic — used for watches and CAS.

## Watches

Client: `watch(/services/payment/, from_revision=R)`

Server tracks watch registrations; on apply, push events to interested watchers.

Don't scan all keys — index watchers by prefix tree.

## Leases & ephemeral keys

```
lease.grant(ttl=10) → lease_id
put(/lock/worker-3, "", lease=lease_id)
keepalive(lease_id) every 3s
```

Lease expiry → delete bound keys automatically. Perfect for **membership** and **lock liveness**.

## Distributed locks

```
txn:
  if create_revision(/locks/job) == 0:
    put(/locks/job, holder_id, lease=lease_id)
  else:
    fail
```

Lock holder must renew lease; on crash, key vanishes → next acquirer wins.

**Fencing token** = mod_revision — pass to downstream storage to reject stale lock holders.

## Storage

Per-node **WAL** (Raft log) + periodic snapshot of full keyspace — your **wal** challenge.

Compact old log entries after snapshot.

## open-crafters tie-in

- **Raft** — leader election, log replication, apply loop
- **Distributed lock** — CAS + lease pattern you've implemented
- **WAL** — durable Raft log segments on each node
