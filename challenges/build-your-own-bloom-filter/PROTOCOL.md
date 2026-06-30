# Wire Protocol — Build your own bloom filter

Build a small **probabilistic set membership** service — the same primitive
behind database query optimizers, CDNs, and distributed caches that need to
answer "might this key exist?" in constant space with no false negatives.

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

Membership uses **k** hash functions derived from FNV-1a 64-bit over the item's
UTF-8 bytes:

```
FNV-1a 64-bit over UTF-8 bytes of item:
  offset basis = 14695981039346656037, prime = 1099511628211
h1 = fnv1a64(item_bytes)
h2 = fnv1a64(item_bytes + byte 0x01)

For i in 0..k-1:
  position_i = (h1 + i * h2) % m

On add: set bits[position_i] = 1
On contains: maybe_present = all bits[position_i] == 1
```

The bit array has **m** bits (indices `0 .. m-1`). Items are UTF-8 strings.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `create`

Create a new bloom filter.

- **params:** `{"filter_id": "<string>", "m": <int>, "k": <int>}`
  - `m` — bit array size, must be **≥ 8**.
  - `k` — number of hash functions, must be **≥ 1**.
- **result:** `{}`
- **errors:**
  - `FILTER_EXISTS` — `filter_id` already exists.
  - `INVALID_PARAMS` — missing fields, `m < 8`, or `k < 1`.

### `add`

Insert an item into a filter (idempotent — adding the same item twice is fine).

- **params:** `{"filter_id": "<string>", "item": "<string>"}` — `item` is UTF-8.
- **result:** `{}`
- **errors:** `FILTER_NOT_FOUND` — unknown `filter_id`.

### `contains`

Probe membership. Bloom filters never produce **false negatives** (an added
item always returns `maybe_present: true`), but may produce **false positives**
(an item never added may still return `true`).

- **params:** `{"filter_id": "<string>", "item": "<string>"}`
- **result:** `{"maybe_present": <bool>}`
- **errors:** `FILTER_NOT_FOUND` — unknown `filter_id`.

### `delete_filter`

Remove a filter from memory (optional but useful for the gauntlet).

- **params:** `{"filter_id": "<string>"}`
- **result:** `{"deleted": true}` if the filter existed and was removed,
  `{"deleted": false}` if there was nothing to delete.

## Error codes

| Code | When |
|---|---|
| `UNKNOWN_METHOD` | Unrecognized `method` |
| `FILTER_EXISTS` | `create` with an existing `filter_id` |
| `FILTER_NOT_FOUND` | `add` / `contains` on an unknown filter |
| `INVALID_PARAMS` | `create` with invalid or missing `m` / `k` |
