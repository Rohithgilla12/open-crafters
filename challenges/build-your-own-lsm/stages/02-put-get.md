# Stage 2: An in-memory key-value store

Before touching disk, get the basic KV semantics right in memory.

## Your task

Implement three RPC methods:

### `put`

```
→ {"id": "1", "method": "put", "params": {"key": "fruit", "value": "apple"}}
← {"id": "1", "result": {}}
```

Overwrites are allowed — a second `put` with the same key replaces the value.

### `get`

```
→ {"id": "2", "method": "get", "params": {"key": "fruit"}}
← {"id": "2", "result": {"value": "apple", "found": true}}

→ {"id": "3", "method": "get", "params": {"key": "missing"}}
← {"id": "3", "result": {"value": null, "found": false}}
```

### `del`

```
→ {"id": "4", "method": "del", "params": {"key": "fruit"}}
← {"id": "4", "result": {"deleted": true}}   // key existed

→ {"id": "5", "method": "del", "params": {"key": "fruit"}}
← {"id": "5", "result": {"deleted": false}}   // key already gone
```

## Tests

The tester writes a key, overwrites it, deletes it, and checks that deleting
one key doesn't affect others. Everything is in memory — no disk yet.

## Notes

- Keys and values are always strings.
- `del` of a missing key is not an error — it returns `deleted: false`.
