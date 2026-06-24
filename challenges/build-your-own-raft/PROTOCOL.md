# Wire Protocol — Build your own Raft

Build a **3-node Raft cluster** that replicates a key-value state machine. The
tester spawns three processes of your program — one per node — and talks to
each over newline-delimited JSON on TCP.

Your program is a **Raft node**. It serves both **client requests** and
**inter-node Raft RPCs** on the same `--port`.

**Prerequisite:** [Build your own WAL](../build-your-own-wal/) teaches
write-before-ack; this challenge applies that discipline to replicated logs.

## Process contract

Each node is started as:

```
./your_program.sh --node-id <id> --peers <peer-list> --port <port> --data-dir <path>
```

- `--node-id` — this node's id (`1`, `2`, or `3` in the tester).
- `--peers` — comma-separated list of **all** cluster members:
  `1=127.0.0.1:1111,2=127.0.0.1:2222,3=127.0.0.1:3333` (includes self).
- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — persistent state directory (required from the durability stage).

Each node must accept connections within **10 seconds** and handle **multiple
concurrent connections**.

## Transport

Newline-delimited JSON, one request line → one response line. Echo the request
`id`. Unknown methods → `UNKNOWN_METHOD`.

## Client methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong", "node_id": "<id>"}`

### `get_status`

- **params:** `{}`
- **result:**
  ```json
  {
    "node_id": "1",
    "role": "leader",
    "term": 2,
    "leader_id": "1",
    "commit_index": 5,
    "last_applied": 5
  }
  ```
  - `role` is one of `leader`, `follower`, `candidate`.
  - `leader_id` is `"0"` when unknown (no leader yet).
  - `commit_index` / `last_applied` are 0 when the log is empty.

### `set`

Replicate a write through Raft. **Only the leader** accepts writes.

- **params:** `{"key": "<string>", "value": <any JSON value>}`
- **result (leader, committed):** `{"index": <log index>}`
- **errors:**
  - `NOT_LEADER` — include `"leader_id": "<id>"` when known (may be `"0"`).
  - `NOT_COMMITTED` — leader could not replicate to a quorum within a
    reasonable time (e.g. partitioned).

The write is **not acknowledged** until a quorum has replicated the entry **and**
it is committed (`commit_index` advanced). Followers must not acknowledge
client writes.

### `get`

Read a key from the **committed, applied** state machine. Any node may serve
reads, but only after applying all entries up to `commit_index`.

- **params:** `{"key": "<string>"}`
- **result:** `{"found": true, "value": <any>}` or `{"found": false}`

## Raft RPC methods (node-to-node)

Same transport. The tester does not call these directly; your nodes must.

### `request_vote`

- **params:**
  ```json
  {
    "term": 2,
    "candidate_id": "2",
    "last_log_index": 5,
    "last_log_term": 2
  }
  ```
- **result:** `{"term": 2, "vote_granted": true}`
- Standard Raft rules: grant at most one vote per term; candidate's log must be
  at least as up-to-date as voter's log.

### `append_entries`

- **params:**
  ```json
  {
    "term": 2,
    "leader_id": "1",
    "prev_log_index": 5,
    "prev_log_term": 2,
    "entries": [{"index": 6, "term": 2, "key": "x", "value": 1}],
    "leader_commit": 5
  }
  ```
  `entries` may be empty (heartbeat).
- **result:** `{"term": 2, "success": true}`
- On failure: `{"term": <current term>, "success": false}`

Log entries contain `index`, `term`, `key`, `value` (SET commands only).

## Raft rules the tester enforces

1. **At most one leader per term** among reachable nodes.
2. **Election:** with three healthy nodes, a leader emerges within **5 seconds**.
3. **Replication:** after a successful `set`, every **running** node has the
   same `commit_index`.
4. **Durability:** after `SIGKILL` of all nodes and restart with the same
   `--data-dir`, committed data survives.
5. **Partition safety:** when the leader is isolated from a quorum, `set` on
   that node must **not** succeed (`NOT_COMMITTED` or `NOT_LEADER`). The
   majority partition must elect a leader and commit new writes.
6. **Election timing:** use election timeouts ≥ **300ms** so tests are stable on
   CI; heartbeats ≤ **150ms**.

## Persistence

From the durability stage onward, survive `SIGKILL` + restart with the same
`--data-dir`: current term, voted-for, log, commit index, and applied KV state.
