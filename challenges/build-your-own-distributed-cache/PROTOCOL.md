# Wire Protocol — Build your own distributed cache

Build a small **in-memory cache node** — the same primitive behind Memcached
shards and Redis nodes: `GET`/`SET` with TTL, conditional writes, batch reads,
and LRU eviction under a memory cap.

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

Keys and values are UTF-8 strings.

## Versioning

Every stored key has a monotonically increasing **version** (integer ≥ 1):

- First successful `set` or `setnx` on a key starts at version **1**.
- Each successful `set` or `cas` swap increments the version.
- `get` and `mget` return the current version on a hit.
- Expired or deleted keys have no version until stored again.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `configure`

Set cache-wide limits. Call before relying on eviction behavior.

- **params:** `{"max_keys": <int>}` — must be **≥ 1**.
- **result:** `{}`
- **errors:** `INVALID_PARAMS` — missing `max_keys` or `max_keys < 1`.

When `max_keys` is configured and a **new** key would exceed the limit, evict
the **least-recently-used** key (see LRU below). Updating an existing key does
not evict other keys.

### `set`

Store a value (create or overwrite).

- **params:** `{"key": "<string>", "value": "<string>", "ttl_ms": <int optional>}`
  - `ttl_ms` optional — if present and **> 0**, the entry expires after that
    many milliseconds (wall clock). Omit or `0` for no expiry.
- **result:** `{"version": <int>}`
- **errors:** `INVALID_PARAMS` — missing `key` or `value`.

### `get`

Fetch a value.

- **params:** `{"key": "<string>"}`
- **result on hit:** `{"hit": true, "value": "<string>", "version": <int>}`
- **result on miss:** `{"hit": false}` (expired keys are misses)
- **errors:** `INVALID_PARAMS` — missing `key`.

A successful `get` on a hit updates LRU recency.

### `delete`

Remove a key immediately.

- **params:** `{"key": "<string>"}`
- **result:** `{"deleted": true}` if the key existed (and was removed),
  `{"deleted": false}` if there was nothing to delete (including expired keys
  already treated as absent).

### `setnx`

Set only if the key is absent (or expired).

- **params:** `{"key": "<string>", "value": "<string>", "ttl_ms": <int optional>}`
- **result on store:** `{"stored": true, "version": <int>}`
- **result on no-op:** `{"stored": false}` — key exists and is not expired
- **errors:** `INVALID_PARAMS` — missing `key` or `value`.

### `cas`

Compare-and-swap on version.

- **params:**
  `{"key": "<string>", "expected_version": <int>, "value": "<string>", "ttl_ms": <int optional>}`
- **result on success:** `{"swapped": true, "version": <int>}` (new version)
- **result on failure:** `{"swapped": false}` — key missing, expired, or
  `expected_version` does not match the current version
- **errors:** `INVALID_PARAMS` — missing fields.

### `mget`

Batch fetch up to **50** keys in one round trip. Order of `entries` matches
the order of `keys` in the request.

- **params:** `{"keys": ["k1", "k2", ...]}`
- **result:** `{"entries": [ ... ]}` where each element is either:
  - `{"key": "<string>", "hit": true, "value": "<string>", "version": <int>}`, or
  - `{"key": "<string>", "hit": false}`
- **errors:** `INVALID_PARAMS` — missing `keys`, empty array, or more than 50 keys.

Each hit in `mget` updates LRU recency for that key.

## LRU eviction

After `configure` sets `max_keys`:

1. `get` and successful `set`/`setnx`/`cas` on an existing key mark it most
   recently used.
2. Inserting a **new** key when the cache already holds `max_keys` entries
   evicts the least-recently-used key before storing the new one.

## Error codes

| Code | When |
|---|---|
| `UNKNOWN_METHOD` | Unrecognized `method` |
| `INVALID_PARAMS` | Missing or invalid parameters |
