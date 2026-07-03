# Stage 3: Head metadata

Downloading a multi-gigabyte object just to learn its size is wasteful. Real
stores expose a metadata-only operation.

## Your task

Implement `head` — like `get`, but **without** returning the body.

```
→ {"id": "1", "method": "head", "params": {"key": "docs/readme.txt"}}
← {"id": "1", "result": {"found": true, "etag": "...", "size": 42}}
```

Missing keys return `NOT_FOUND`, same as `get`.

## Tests

- After a `put`, `head` returns the correct `etag` and `size`.
- The response must **not** include a `body` field (or it must be empty).
- `head` on a missing key returns `NOT_FOUND`.

## Notes

- You can share almost all of the lookup logic with `get` — just omit the
  body from the result.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Reuse the same lookup as `get`, but omit `body` from the result. Still error with `NOT_FOUND` when the key is absent.

Or run: <code>crafters hint object-store --stage head</code>
</details>
<!-- /crafters-stage-hint -->
