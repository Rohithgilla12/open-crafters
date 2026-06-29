# Wire Protocol — Build your own object store

Build a small **object storage service** — the primitive behind S3, GCS, and
every blob store in production. Clients `put` and `get` opaque blobs keyed by
string paths, list by prefix, upload large objects in parts, and rely on your
server to keep bytes durable across crashes.

The tester grades you entirely over TCP — and by `SIGKILL`ing your process to
verify that acknowledged writes survive restart with the same `--data-dir`.

Unlike the WAL challenge, the on-disk format is **not** graded: persist however
you like inside `--data-dir`. The whole contract is *behavioral*.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for durable state. From the **Durability** stage on,
  stored objects (and in-progress multipart uploads) must survive `SIGKILL` +
  restart with the same `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

Keys are arbitrary strings (think `photos/2024/cat.jpg`). There is no bucket
abstraction — the key namespace is flat.

## ETags

Every stored object has an **etag**: the lowercase hexadecimal SHA-256 digest
of the object's raw body bytes. `put` returns it; `get` and `head` echo it.
Multipart part uploads return a per-part etag (SHA-256 of that part's bytes);
`complete_multipart` returns the etag of the assembled object.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `put`

Store (or replace) an object.

- **params:** `{"key": "<string>", "body": "<string>"}`
- **result:** `{"etag": "<hex>"}` — SHA-256 of `body` as described above.
- **durability:** from the Durability stage on, only return the etag after the
  object is durably stored.

### `get`

Fetch an object's body and metadata.

- **params:** `{"key": "<string>"}`
- **result (found):** `{"found": true, "body": "...", "etag": "...", "size": <int>}`
  where `size` is the byte length of `body`.
- **errors:** `NOT_FOUND` — no object at `key`.

### `head`

Like `get`, but **without** the body — metadata only.

- **params:** `{"key": "<string>"}`
- **result (found):** `{"found": true, "etag": "...", "size": <int>}`
- **errors:** `NOT_FOUND` — no object at `key`.

### `delete`

Remove an object.

- **params:** `{"key": "<string>"}`
- **result:** `{"deleted": true}` if the key existed and was removed,
  `{"deleted": false}` if there was nothing to delete.

### `list`

List object keys matching a prefix.

- **params:** `{"prefix": "<string>"}` — use `""` to list all keys.
- **result:** `{"keys": ["...", ...]}` — keys whose names **start with**
  `prefix`, sorted in **lexicographic** (byte/string) order. No matches →
  `{"keys": []}`.

### `create_multipart`

Begin a multipart upload for a large object.

- **params:** `{"key": "<string>"}`
- **result:** `{"upload_id": "<opaque-id>"}`

The upload is not visible via `get` until `complete_multipart` succeeds.

### `upload_part`

Upload one part of a multipart upload.

- **params:**
  `{"upload_id": "<id>", "part_number": <int>, "body": "<string>"}`
  — `part_number` is 1-based.
- **result:** `{"etag": "<hex>"}` — SHA-256 of this part's body.
- **errors:** `NO_SUCH_UPLOAD` — unknown `upload_id`.

You may upload parts in any order; the same `part_number` may be overwritten
before completion.

### `complete_multipart`

Assemble uploaded parts into the final object at the key from
`create_multipart`.

- **params:**
  `{"upload_id": "<id>", "parts": [{"part_number": <int>, "etag": "<hex>"}, ...]}`
- **result:** `{"etag": "<hex>"}` — SHA-256 of the assembled body.
- **assembly:** concatenate part bodies in **ascending `part_number` order**
  (the `parts` array in the request must also be sorted by `part_number`).
- **errors:**
  - `NO_SUCH_UPLOAD` — unknown or already-completed `upload_id`.
  - `INVALID_PART` — a part is missing, its etag does not match what was
    uploaded, or the `parts` list is not in ascending `part_number` order.

After a successful complete, the upload is consumed and the object is readable
via `get` / `head`.

## Durability

From the Durability stage on:

- An acknowledged `put` survives `SIGKILL` and restart.
- An acknowledged `delete` stays deleted across a crash.
- A completed multipart upload's object survives restart.
- In-progress multipart uploads (parts uploaded but not yet completed) should
  also survive if you have persisted them — the gauntlet may crash mid-upload.

Write durability as if the power can fail after any syscall — fsync (or
equivalent) before you acknowledge. The tester checks everything it can
observe; the fsync honor system applies as in the WAL challenge.

## Error codes

| Code | When |
|---|---|
| `UNKNOWN_METHOD` | Unrecognized `method` |
| `NOT_FOUND` | `get` / `head` on a missing key |
| `NO_SUCH_UPLOAD` | Multipart op on unknown `upload_id` |
| `INVALID_PART` | `complete_multipart` with wrong etag, order, or missing part |
