# Wire Protocol — Build your own hash ring

Build a small **consistent hash ring** service — the same placement primitive
behind distributed caches, object stores, and shard routers that need to add
or remove nodes without reshuffling every key.

The tester grades you entirely over TCP. All state is **in-memory**; there is
no durability stage and no `--data-dir`.

## Process contract

```
./your_program.sh --port <port>
```

- `--port` — TCP port to listen on (`127.0.0.1`).

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections. The harness may pass `--data-dir` as well; you may
ignore it.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

## Hash specification

```
FNV-1a 64-bit: offset=14695981039346656037, prime=1099511628211

hash_key(key) = fnv1a64(utf8(key))

For physical node_id and replica index i (0 <= i < replicas):
  vnode_position = fnv1a64(utf8(node_id + "#" + decimal(i)))

Lookup:
  - Collect all vnode positions on the ring, each tagged with its node_id
  - Sort positions ascending (uint64). Ties: lexicographically smaller node_id wins
  - h = hash_key(key)
  - Choose the vnode with the smallest position >= h, or wrap to the smallest position if none
  - Return that vnode's node_id
```

Keys and node IDs are UTF-8 strings. `replicas` is the virtual-node count per
physical node (integer **≥ 1**), fixed when the ring is created.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `create_ring`

Create a new hash ring.

- **params:** `{"ring_id": "<string>", "replicas": <int>}`
  - `replicas` — virtual nodes per physical node, must be **≥ 1**.
- **result:** `{}`
- **errors:**
  - `RING_EXISTS` — `ring_id` already exists.
  - `INVALID_PARAMS` — missing fields or `replicas < 1`.

### `add_node`

Add a physical node to a ring (creates `replicas` vnodes).

- **params:** `{"ring_id": "<string>", "node_id": "<string>"}`
- **result:** `{}`
- **errors:**
  - `RING_NOT_FOUND` — unknown `ring_id`.
  - `NODE_EXISTS` — `node_id` already on the ring.

### `remove_node`

Remove a physical node and its vnodes.

- **params:** `{"ring_id": "<string>", "node_id": "<string>"}`
- **result:** `{"removed": true}` if the node existed and was removed,
  `{"removed": false}` if it was not on the ring.
- **errors:** `RING_NOT_FOUND` — unknown `ring_id`.

### `lookup`

Find the owning physical node for a key.

- **params:** `{"ring_id": "<string>", "key": "<string>"}`
- **result:** `{"node_id": "<string>"}`
- **errors:**
  - `RING_NOT_FOUND` — unknown `ring_id`.
  - `NO_NODES` — ring has no physical nodes.

### `list_nodes`

List physical nodes on a ring.

- **params:** `{"ring_id": "<string>"}`
- **result:** `{"nodes": ["...", ...]}` — physical node IDs sorted
  **lexicographically ascending**.
- **errors:** `RING_NOT_FOUND` — unknown `ring_id`.

## Error codes

| Code | When |
|---|---|
| `UNKNOWN_METHOD` | Unrecognized `method` |
| `RING_EXISTS` | `create_ring` with an existing `ring_id` |
| `RING_NOT_FOUND` | Operation on an unknown ring |
| `NODE_EXISTS` | `add_node` with an existing `node_id` |
| `INVALID_PARAMS` | `create_ring` with invalid or missing params |
| `NO_NODES` | `lookup` on an empty ring |
