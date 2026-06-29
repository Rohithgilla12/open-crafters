# Stage 6: List by prefix

Object stores are not just a single key — clients enumerate keys under a
prefix, like listing `photos/2024/` without knowing every filename in advance.

## Your task

Implement `list`.

```
→ {"id": "1", "method": "list", "params": {"prefix": "a/"}}
← {"id": "1", "result": {"keys": ["a/1", "a/10", "a/2"]}}
```

- Only keys that **start with** the prefix are returned.
- Keys are sorted in **lexicographic** (string) order.
- An empty prefix lists all keys.
- No matches → `{"keys": []}`.

## Tests

- Several objects under different prefixes; `list` with `prefix: "a/"` returns
  only `a/*` keys in lexicographic order (`a/10` before `a/2`).
- `prefix: ""` returns every key, sorted.
- A prefix with no matches returns an empty array.

## Notes

- Lexicographic order is byte/string order, not numeric — `10` sorts before `2`.
