# Stage 7: Multipart upload

Large objects are uploaded in parts and assembled server-side — the pattern
behind S3 multipart uploads.

## Your task

Implement three methods:

**`create_multipart`** — start an upload for a key.

```
← {"result": {"upload_id": "abc123"}}
```

**`upload_part`** — upload one part (1-based `part_number`).

```
← {"result": {"etag": "<sha256 of part body>"}}
```

**`complete_multipart`** — assemble parts into the final object.

```
→ {"params": {"upload_id": "abc123", "parts": [
     {"part_number": 1, "etag": "..."},
     {"part_number": 2, "etag": "..."}
   ]}}
← {"result": {"etag": "<sha256 of concatenated bodies>"}}
```

The assembled body is the concatenation of part bodies in **ascending
`part_number` order**. The `parts` array in `complete_multipart` must also be
sorted by `part_number`.

## Errors

- `NO_SUCH_UPLOAD` — bad `upload_id`.
- `INVALID_PART` — wrong etag, missing part, or `parts` not in ascending
  `part_number` order.

## Tests

- Upload three parts, complete with correct etags in order → object readable
  via `get` with the assembled body and final etag.
- Wrong etag or wrong order in `complete_multipart` → `INVALID_PART`.
- Unknown `upload_id` → `NO_SUCH_UPLOAD`.
- Completing the same upload twice → `NO_SUCH_UPLOAD` the second time.

## Notes

- The object is not visible at the key until `complete_multipart` succeeds.
- Parts may be uploaded in any order before completion.
