# Stage 4: Overwrite

Keys are unique. Writing again at the same key replaces the previous object —
there is no versioning in this challenge.

## Your task

Make a second `put` to the same key replace the stored bytes and return a **new**
etag reflecting the new body.

## Tests

- `put` key `K` with body `A`, then `put` key `K` with body `B`.
- The two etags must differ.
- `get` on `K` returns body `B` and the second etag.

## Notes

- This is just map assignment — but the etag must change with the content.
