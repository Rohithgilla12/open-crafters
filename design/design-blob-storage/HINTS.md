# Hints — blob storage

Open only if you're stuck.

## Metadata vs data

**Metadata** (small, indexed): `bucket, key → {size, etag, storage_nodes[], version, timestamps}`

**Data** (large, immutable): raw bytes on storage nodes — your **object store** challenge already models this.

Never put blob bytes in a SQL row.

## Small PUT path

```
Client ──▶ API gateway ──▶ metadata service (insert index row)
                              │
                              └── stream bytes to storage node(s)
```

Write to **WAL** / journal before acking metadata — crash recovery replays in-flight writes.

## Large PUT (multipart)

1. `CreateMultipartUpload` → `upload_id`
2. `UploadPart(n, bytes)` → store part blob + record part etag
3. `CompleteMultipartUpload` → assemble metadata row pointing at ordered parts
4. Background job concatenates or serves as logical byte ranges over parts

## Placement

**Hash ring** maps `hash(bucket, key)` → storage node set. Replication factor 3 across failure domains.

## GET path

Lookup metadata → fetch from nearest replica → stream with sendfile.

Range reads: compute which part/offset → HTTP 206 Partial Content.

## Listing

Prefix scan on `(bucket, key)` index — not a full table scan.

`ListObjectsV2` with delimiter `/` simulates folders via common prefix grouping.

## Delete

Metadata tombstone + async data node GC. Versioned buckets keep prior version rows.

## open-crafters tie-in

- **Object store** — PUT/GET/DELETE/Multipart protocol you've implemented
- **WAL** — metadata journal for crash-safe index updates
- **Hash ring** — object placement and rebalancing across storage nodes
