# Stage 4: Write the log format

Now the actual WAL. From this stage on, your durability mechanism is an
**append-only log** at `<data-dir>/wal.log`, in a format specified to the
byte — because the tester opens the file and parses it.

## Your task

Append exactly one record per acknowledged `set` or `del`, in
acknowledgement order. Each record is:

```
crc (4 bytes, LE) | length (4 bytes, LE) | payload (length bytes)
```

- `payload` is UTF-8 JSON: `{"op": "set", "key": "...", "value": "..."}` or
  `{"op": "del", "key": "..."}`.
- `length` is the payload's byte length, little-endian.
- `crc` is **CRC-32 (IEEE)** — the standard `crc32` in zlib, Python's
  `zlib.crc32`, Go's `hash/crc32.ChecksumIEEE` — computed over the 4
  `length` bytes followed by the payload, stored little-endian.
- No file header: byte 0 of `wal.log` is the first record's CRC.

Recovery (which you built in stage 3) now means: replay `wal.log` from the
start, applying each operation in order.

## Tests

The tester performs `set fruit=apple`, `set color=blue`, `del fruit`,
`set color=green`, then — **without killing your process** — parses
`wal.log` and expects exactly those 4 records, byte-valid CRCs, in order.
Checking the file while you're still running is deliberate: it verifies the
record was on disk *before* you acknowledged.

## Notes

- Why does the CRC cover the `length` field too? Because a torn write can
  mangle the header itself: a corrupted length would send a naive reader off
  into the void. Covering it means any damage to the frame is detectable.
  Stages 6 and 7 weaponize exactly this.
- Keep one append handle open; don't reopen the file per write.
- Yes, a `del` of a missing key still appends a record — determinism over
  cleverness (and the tester checks).
