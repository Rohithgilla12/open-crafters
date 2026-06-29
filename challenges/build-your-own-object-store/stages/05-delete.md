# Stage 5: Delete

Objects should not live forever. Clients need a way to remove a key.

## Your task

Implement `delete`.

```
→ {"id": "1", "method": "delete", "params": {"key": "tmp/scratch"}}
← {"id": "1", "result": {"deleted": true}}     (key existed)
← {"id": "2", "result": {"deleted": false}}    (nothing to delete)
```

After a successful delete, `get` and `head` on that key must return
`NOT_FOUND`.

## Tests

- Deleting a missing key returns `deleted: false` (not an error).
- Deleting an existing key returns `deleted: true` and removes the object.
- A second delete of the same key returns `deleted: false`.

## Notes

- Deletes are idempotent at the protocol level — missing keys are not errors.
