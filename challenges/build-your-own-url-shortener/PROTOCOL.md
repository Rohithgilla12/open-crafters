# Wire Protocol — Build your own URL shortener (gateway)

You implement the **gateway** only. The harness spawns three **reference**
services (id-generator, bloom-filter, object-store) and injects their addresses
into your environment before starting your program.

## Your process

```
./your_program.sh --port <port>
```

Environment variables set by the harness:

| Variable | Service |
|----------|---------|
| `IDGEN_ADDR` | Reference `build-your-own-id-generator` |
| `BLOOM_ADDR` | Reference `build-your-own-bloom-filter` |
| `STORE_ADDR` | Reference `build-your-own-object-store` |

Read each child’s existing `PROTOCOL.md` in those challenges — speak NDJSON to
those addresses over TCP. Your gateway exposes a **new** protocol below.

## Gateway transport

Newline-delimited JSON, same shape as other challenges.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `shorten`

Mint a short code for a long URL.

- **params:** `{"url": "<string>"}`
- **result:** `{"code": "<string>"}`

Expected orchestration:

1. `next_id` on id-generator → use decimal `id` as `code`
2. `add` code to bloom filter `codes`
3. `put` `links/<code>` → url in object store

### `resolve`

Look up a code.

- **params:** `{"code": "<string>"}`
- **result (hit):** `{"found": true, "url": "<string>"}`
- **result (miss):** `{"found": false}`

Expected orchestration:

1. `contains` on bloom filter `codes` — if `maybe_present: false`, return miss
2. `get` `links/<code>` from object store — return url or miss

### `record_click`

Record analytics for a code.

- **params:** `{"code": "<string>"}`
- **result:** `{}`

Store a click object at key `clicks/<code>` in the object store (any non-empty
body is fine).

## Bloom filter setup

On first use, `create` filter `codes` with `m >= 8192`, `k >= 4` (ignore
`FILTER_EXISTS` if already created).

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognized gateway method |
| `INVALID_PARAMS` | Missing `url` / `code` |

Child service errors may propagate as gateway `INTERNAL` or be mapped to misses
where appropriate.
