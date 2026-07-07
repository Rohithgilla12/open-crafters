# Walkthrough — Build your own cache cluster

Meta-compose gateway — route keys across two cache nodes with bloom + rate limiter.

## bind — Boot the stack

> **Hint:** Read all five env addresses. Gateway only needs `ping`; harness pings references directly.

## set-get — Set and get

> **Hint:** `lookup` ring `cache` → cache `set` on the right node → bloom `add` on `keys`.

## delete — Delete a key

> **Hint:** Same routing as `set`, then cache `delete`.

## routing — Shard routing

> **Hint:** Map `node1` → `CACHE_NODE1_ADDR`, `node2` → `CACHE_NODE2_ADDR`.

## bloom — Bloom membership

> **Hint:** `add` every successfully stored key to filter `keys`.

## bloom-miss — Fast negative lookup

> **Hint:** If bloom `contains` is false, return `hit: false` without calling cache.

## rate-limit — Stampede guard

> **Hint:** `take` on `rl:<key>` before cache I/O; return `RATE_LIMITED` when `allowed: false`.

## mget — Multi-get

> **Hint:** Loop keys with the same logic as `get`.

## gauntlet — The gauntlet

> **Hint:** Reuse set/get/delete; support optional `ttl_ms` on `set`.
