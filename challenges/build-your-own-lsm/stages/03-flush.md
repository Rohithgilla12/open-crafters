# Stage 3: Flush to SSTable

Now make writes durable. The memtable is fast but volatile — `flush` writes
it to an immutable SST file on disk.

## Your task

Implement `flush`:

```
→ {"id": "1", "method": "flush", "params": {}}
← {"id": "1", "result": {}}
```

**Effects:**

1. Write the entire memtable to `<data-dir>/sst/000001.sst` (see
   [PROTOCOL.md](../PROTOCOL.md) for the byte-exact SST1 format).
2. Entries must be sorted lexicographically by key.
3. Clear the memtable.
4. Fsync the file before acknowledging.

The first flush creates `000001.sst`, the second creates `000002.sst`, and so
on (6-digit zero-padded sequence numbers).

## SST1 format (graded byte-for-byte)

```
magic "SST1" (4 bytes)
uint32 entry_count LE
repeat:
  uint32 key_len LE
  key bytes
  uint32 value_len LE
  value bytes
```

## Tests

The tester puts three keys, calls `flush`, **kills your process**, and parses
`000001.sst` directly — without restarting you. The file must contain exactly
those three keys in sorted order, conforming to SST1.

## Notes

- Only `flush` makes data durable. Unflushed memtable contents are lost on
  crash.
- Create `<data-dir>/sst/` if it doesn't exist.
