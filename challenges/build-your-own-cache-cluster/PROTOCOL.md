# Wire Protocol — Build your own cache cluster (gateway)

You implement the **client gateway** only. The harness spawns five **reference**
services (hash-ring, bloom-filter, rate-limiter, and two cache nodes) and
injects their addresses into your environment.

## Your process

```
./your_program.sh --port <port>
```

| Variable | Service |
|----------|---------|
| `HASHRING_ADDR` | Reference `build-your-own-hash-ring` |
| `BLOOM_ADDR` | Reference `build-your-own-bloom-filter` |
| `LIMITER_ADDR` | Reference `build-your-own-rate-limiter` |
| `CACHE_NODE1_ADDR` | Reference `build-your-own-distributed-cache` (`node1`) |
| `CACHE_NODE2_ADDR` | Reference `build-your-own-distributed-cache` (`node2`) |

## Setup (first use)

1. `create_ring` id `cache` with `replicas >= 1`
2. `add_node` `node1` and `node2`
3. `create` bloom filter `keys` (`m >= 8192`, `k >= 4`)
4. `configure` each cache node with a generous `max_keys`

## Routing

`lookup` on ring `cache` → `node_id` → TCP to `CACHE_NODE1_ADDR` or
`CACHE_NODE2_ADDR`.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `set`

- **params:** `{"key": "<string>", "value": "<string>", "ttl_ms": <int optional>}`
- **result:** `{"version": <int>}`

Orchestration: rate-limit admit → ring lookup → cache `set` → bloom `add`.

### `get`

- **params:** `{"key": "<string>"}`
- **result (hit):** `{"hit": true, "value": "<string>", "version": <int>}`
- **result (miss):** `{"hit": false}`

Orchestration: bloom `contains` — if `maybe_present: false`, return miss without
touching cache. Otherwise rate-limit admit → lookup → cache `get`.

### `delete`

- **params:** `{"key": "<string>"}`
- **result:** `{"deleted": true|false}`

Rate-limit admit → lookup → cache `delete`. (Bloom may still report
`maybe_present` — that is fine.)

### `mget`

- **params:** `{"keys": ["k1", "k2", ...]}`
- **result:** `{"entries": [{"key":"k1","hit":true,"value":"...","version":1}, {"key":"k2","hit":false}, ...]}`

Per-key `get` semantics in request order.

## Rate limiting

Before cache access on `get`/`set`/`delete`, call limiter `take` on key
`rl:<key>`. If the limiter is missing, `configure` a generous token bucket
(capacity 100). If `take` returns `allowed: false`, error with code
`RATE_LIMITED`.

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognized gateway method |
| `INVALID_PARAMS` | Missing fields |
| `RATE_LIMITED` | Limiter denied the operation |
