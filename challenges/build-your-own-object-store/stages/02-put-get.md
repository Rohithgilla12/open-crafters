# Stage 2: Put and get

Object storage is keyed blobs. A client `put`s bytes at a path-like key and
`get`s them back later. This stage is the happy path, in memory.

## Your task

Implement `put` and `get`.

**`put`** — store an object, return its etag (lowercase hex SHA-256 of the
body bytes).

```
→ {"id": "1", "method": "put", "params": {"key": "photos/cat.jpg", "body": "meow"}}
← {"id": "1", "result": {"etag": "a1b2..."}}
```

**`get`** — fetch a stored object.

```
← {"id": "2", "result": {"found": true, "body": "meow", "etag": "a1b2...", "size": 4}}
```

A missing key is an error:

```
← {"id": "3", "error": {"code": "NOT_FOUND", "message": "..."}}
```

## Tests

- `put` returns the correct SHA-256 etag for the body.
- `get` after `put` returns `found: true`, the same body, etag, and `size`
  equal to the byte length.
- `get` on a key that was never written returns `NOT_FOUND`.

## Notes

- Etags are **content hashes**, not random ids. The tester will verify the
  exact digest.
- Guard shared state with a lock if connections run concurrently — you'll need
  it later anyway.
