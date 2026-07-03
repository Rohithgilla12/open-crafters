# Reference architecture — blob storage

Spoiler: compare after your own attempt.

## Components

```
S3 API ──▶ Request router
               │
       ┌───────┴───────┐
       ▼               ▼
  Metadata cluster   Storage nodes (data plane)
  (index + WAL)      (replicated blobs)
       │
       └── CDN / edge cache (optional hot GET)
```

## Metadata schema

```
objects: (bucket, key, version_id) → {
  size, etag, content_type,
  part_refs[] | inline_blob_ref,
  storage_tier, created_at, deleted
}

multipart_uploads: upload_id → { bucket, key, parts[{n, etag, node_ref}] }
```

Index backed by distributed KV or LSM — range scans on `(bucket, key)` prefix.

## Small object PUT

1. Authenticate; check bucket quota.
2. Stream body; compute etag on the fly.
3. **WAL** append: `PutObject(bucket, key, size, etag, node_set)`.
4. Replicate bytes to RF=3 storage nodes via hash ring placement.
5. Commit metadata; return `200` + etag.

Idempotent PUT with same key replaces prior version (new version_id if versioning on).

## Multipart PUT

```
CreateMultipartUpload ──▶ upload_id
       │
UploadPart × N ──▶ part blobs on storage nodes (≥ 5 MB each)
       │
CompleteMultipartUpload ──▶ metadata row with ordered part refs
       │
Background assembler (optional) ──▶ single concatenated blob for cold tier
```

AbortMultipartUpload → mark upload cancelled; GC reclaims parts after TTL.

## GET path

1. Lookup metadata by `(bucket, key)` — **strong consistency** on latest version.
2. Resolve storage node(s) from ring; pick nearest healthy replica.
3. Stream bytes; honor `Range` header via offset math on parts.
4. CDN caches immutable objects keyed by etag.

| Object size | Strategy |
|-------------|----------|
| &lt; 1 MB | Inline or single node |
| 1 MB – 5 GB | Single blob, range-friendly |
| &gt; 5 GB | Multipart only; parallel part fetch |

## Replication & durability

```
         hash(bucket:key)
               │
    ┌──────────┼──────────┐
    ▼          ▼          ▼
  Node A     Node B     Node C   (RF=3, different racks/AZs)
```

Erasure coding (RS 6+3) optional for cold tier — trades CPU for storage cost.

Background **scrubber** verifies checksums; repairs from healthy replica.

## ListObjects

```
SELECT * FROM objects
  WHERE bucket = ? AND key > ? AND key LIKE 'prefix%'
  ORDER BY key LIMIT 1000
```

Continuation token = last key returned. Delimiter `/` groups `common_prefixes`.

## Delete & GC

1. Metadata soft-delete (tombstone) or version bump.
2. Async GC on storage nodes removes unreferenced blobs.
3. Multipart part sweeper deletes abandoned uploads &gt; 7 days.

## Capacity

| Layer | Sizing |
|-------|--------|
| Metadata | ~500 bytes/object → 50 PB index at 100T objects (sharded) |
| Storage nodes | 100 TB/disk × N nodes; sequential write friendly |
| API tier | Stateless; scale on GET QPS |

## Comparison table

| System | Metadata | Data placement |
|--------|----------|----------------|
| S3 | Dynamo-style partition index | Hash-based, RF across AZs |
| GCS | Colossus metadata | Reed-Solomon + replication |
| MinIO | Erasure sets per pool | Consistent hash within pool |

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Blob PUT/GET/Multipart API | `build-your-own-object-store` |
| Crash-safe metadata updates | `build-your-own-wal` |
| Object → node placement | `build-your-own-hash-ring` |

Blob storage splits **small consistent metadata** from **large immutable data** — your object-store challenge is the data plane; this problem adds the control plane.
