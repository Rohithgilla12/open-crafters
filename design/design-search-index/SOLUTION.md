# Reference architecture — search index

## Pipeline

```
Product DB ──CDC──▶ Kafka ──▶ Indexers ──▶ Shard writers
                                    │
User query ──▶ Query coordinator ────┴──▶ merge & rank
```

## On-disk layout (per shard)

- **Memtable**: recent docs in RAM.
- **Immutable segments**: sorted term → compressed posting lists.
- **Bloom filters** per segment: term definitely absent → skip IO.

Same compaction story as your **LSM** — merge segments, drop deleted docs.

## Query execution

1. Parse query → AST (must, should, filter).
2. For each shard in parallel: local top 20 by score.
3. Coordinator merges 20×N → global top 20.
4. Fetch snippets from doc store (or stored fields).

## Doc store

Search index has IDs + scores; full `body` may live in separate KV keyed by `doc_id` for large payloads.

## Near real-time

Refresh interval (e.g. 1s): memtable flush makes new docs visible without full segment rebuild.

## Autocomplete service

Edge n-grams at index time: `"quick"` → prefixes `q`, `qu`, `qui`…

Low-latency KV or dedicated small shards. Updated from same CDC stream.

## Deletes

`tombstone(doc_id)` in segment. Queries filter tombstoned IDs. Merge garbage-collects.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Change stream | `build-your-own-log` |
| Segment storage | `build-your-own-lsm` |
| Segment skip lists | `build-your-own-bloom-filter` |

Search is an LSM of inverted lists plus a distributed merge on read.
