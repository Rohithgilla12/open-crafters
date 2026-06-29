# Build your own object store

Build the **object storage service** — the primitive behind S3, GCS, MinIO,
and every system that stores opaque blobs keyed by path-like strings.

The vehicle is a small TCP server, but the substance is real object-store
behavior:

- **put / get** with content-addressed **etags** (SHA-256),
- **head** for metadata without transferring bytes,
- **overwrite** and **delete** semantics,
- **prefix listing** in lexicographic order,
- **multipart upload** for large objects assembled from ordered parts,
- **durability** across `SIGKILL`, and
- a final **gauntlet** mixing every operation with repeated crashes.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [Put and get](stages/02-put-get.md) | `put` / `get`, SHA-256 etags |
| 3 | [Head metadata](stages/03-head.md) | `head` without body |
| 4 | [Overwrite](stages/04-overwrite.md) | second `put` replaces |
| 5 | [Delete](stages/05-delete.md) | `delete` |
| 6 | [List by prefix](stages/06-list.md) | `list` |
| 7 | [Multipart upload](stages/07-multipart.md) | create / upload / complete |
| 8 | [Survive a crash](stages/08-durability.md) | persist to `--data-dir` |
| 9 | [The gauntlet](stages/09-gauntlet.md) | mixed ops + SIGKILL |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) for the full wire contract. Copy a starter from
[starters/](starters/), then:

```sh
./crafters grade --challenge build-your-own-object-store \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-object-store/go/](../../examples/solutions/build-your-own-object-store/go/).
