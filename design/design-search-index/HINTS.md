# Hints — search index

## Inverted index

For each term `t`: posting list of `(doc_id, positions, boosts)`.

On disk: segmented **LSM-style** immutable segments + in-memory buffer — your **LSM** challenge intuition.

## Ingestion

CDC / **log** from product DB → consumers parse docs → batch write to index shard.

## Sharding

`shard = hash(doc_id) % N`. Query fans out to all shards (or routing by filter), merge top-K.

## Updates

Mark old doc_id deleted (tombstone); insert new version. Periodic segment merge compacts tombstones.

## Autocomplete

Separate **prefix trie** or edge n-gram index — smaller, optimized for `prefix*` queries.

## open-crafters tie-in

- **Log** — document change stream
- **LSM** — segment storage on disk
- **Bloom filter** — skip segments without term
