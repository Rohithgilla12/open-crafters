# Build your own distributed cache

Build an **in-memory cache node** — the primitive behind Memcached shards and
Redis nodes: fast `GET`/`SET`, TTL expiration, conditional writes, batch reads,
and LRU eviction when memory is capped.

The vehicle is a small TCP server; the substance is real cache semantics:

- **set** / **get** with monotonic **versions**,
- **delete** and **ttl_ms** expiration,
- **setnx** (create-only) and **cas** (compare-and-swap),
- **mget** for batch reads,
- **configure** `max_keys` with **LRU** eviction, and
- a final **gauntlet** of concurrent churn.

All state is in-memory — no durability stage.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [Set and get](stages/02-set-get.md) | `set`, `get`, versions |
| 3 | [Delete a key](stages/03-delete.md) | `delete` |
| 4 | [TTL expiration](stages/04-ttl.md) | `ttl_ms` wall-clock expiry |
| 5 | [Set if not exists](stages/05-setnx.md) | `setnx` |
| 6 | [Compare and swap](stages/06-cas.md) | `cas` on `expected_version` |
| 7 | [Multi-get](stages/07-mget.md) | `mget` batch |
| 8 | [LRU eviction](stages/08-lru.md) | `configure` + LRU under `max_keys` |
| 9 | [The gauntlet](stages/09-gauntlet.md) | concurrent connections + TTL/setnx |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) for the full wire contract. Copy a starter from
[starters/](starters/), then:

```sh
./crafters grade --challenge build-your-own-distributed-cache \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-distributed-cache/go/](../../examples/solutions/build-your-own-distributed-cache/go/).

Pair with the [distributed cache design problem](https://learn.gilla.fun/design/design-distributed-cache) — whiteboard the cluster first, then implement the node.
