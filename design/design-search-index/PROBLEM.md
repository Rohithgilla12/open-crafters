# Design a search index

Design **Elasticsearch-class** search for a product catalog or document corpus: users type queries, get ranked results in milliseconds.

## Functional requirements

1. **Index document** — add/update/delete searchable docs with fields (title, body, tags).
2. **Search** — full-text query with filters (`category:shoes price:&lt;100`).
3. **Autocomplete** — prefix suggestions as user types.
4. **Ranking** — relevance score + business boosts (sponsored, popularity).
5. **Near real-time** — new docs searchable within **&lt; 30s**.

## Scale

| Metric | Value |
|--------|-------|
| Documents | 500M |
| Index size | 20 TB |
| Queries / sec | 50k peak |
| Writes / sec | 10k peak (indexing) |
| Avg doc | 2 KB text |

## Non-functional

- Query p99 **&lt; 100ms**.
- Indexing must not stall queries.
- Deletions must not resurrect in results.

## Your task

Whiteboard **40–50 minutes**:

1. Ingestion pipeline from source of truth.
2. Inverted index structure (terms → posting lists).
3. Sharding strategy and cross-shard merge.
4. Update/delete handling (tombstones vs segments).
5. Autocomplete — same cluster or separate?

## Stretch

- Faceted navigation (count per category).
- Personalization without breaking p99.
