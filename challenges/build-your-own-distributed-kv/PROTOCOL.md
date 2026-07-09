# Wire Protocol — Build your own distributed KV (gateway)

You implement the **client gateway** only. The harness spawns three **reference**
services and injects their addresses into your environment before starting your
program.

## Your process

```
./your_program.sh --port <port>
```

| Variable | Service |
|----------|---------|
| `HASHRING_ADDR` | Reference `build-your-own-hash-ring` |
| `RAFT1_ADDR` | Raft cluster node 1 (`build-your-own-raft`) |
| `RAFT2_ADDR` | Raft cluster node 2 |
| `RAFT3_ADDR` | Raft cluster node 3 |
| `LSM_ADDR` | Reference `build-your-own-lsm` |

Read each child's `PROTOCOL.md` — speak NDJSON over TCP to those addresses.
Your gateway exposes a **new** protocol below.

## Architecture

Two logical shards on one hash ring (`ring_id`: **`kv`**, `replicas`: **64**):

| Physical node | Storage | Role |
|---------------|---------|------|
| `raft-shard` | 3-node Raft cluster | Replicated writes via leader `set` / reads on any node |
| `lsm-shard` | LSM engine | Single-node shard with `put` / `get` / `del` / `flush` |

On boot (before serving traffic), initialize the ring:

1. `create_ring` — `ring_id=kv`, `replicas=64`
2. `add_node` — `raft-shard` and `lsm-shard`

For each client request:

1. `lookup` on the hash ring → `raft-shard` or `lsm-shard`
2. Forward to the correct backend (`set`/`get` on Raft leader, `put`/`get`/`del` on LSM)

Raft writes must go through the **leader** — retry other nodes on `NOT_LEADER`.

## Gateway transport

Newline-delimited JSON, same shape as other challenges.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `put`

- **params:** `{"key": "<string>", "value": "<string>"}`
- **result:** `{}`
- Routes via hash ring; Raft shard uses `set`, LSM shard uses `put`.

### `get`

- **params:** `{"key": "<string>"}`
- **result:** `{"found": true, "value": "<string>"}` or `{"found": false, "value": null}`

### `delete`

- **params:** `{"key": "<string>"}`
- **result:** `{"deleted": true}` or `{"deleted": false}`
- **LSM shard only** — return error `UNSUPPORTED` for keys on `raft-shard`.
