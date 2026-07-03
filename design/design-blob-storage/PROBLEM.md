# Design a blob storage service

Design an **S3 / GCS-class** object store: clients upload, download, list, and delete opaque blobs keyed by bucket + object name.

## Functional requirements

1. **PutObject** — store bytes at `s3://bucket/key`; return etag (content hash) and version id.
2. **GetObject** — stream object by bucket + key; support byte-range reads (`Range: bytes=0-1023`).
3. **DeleteObject** — remove object; versioning may retain prior versions.
4. **ListObjects** — prefix listing with pagination (`continuation_token`).
5. **Multipart upload** — upload large objects in parts; complete or abort; parts ≥ 5 MB.
6. **Buckets** — namespace isolation; per-bucket policies and quotas.

## Scale

| Metric | Value |
|--------|-------|
| Objects | 100T |
| Total stored bytes | 500 EB |
| Peak PUT rate | 500k objects/sec |
| Peak GET rate | 10M objects/sec |
| Avg object size | 5 MB (heavy tail: 1 KB metadata, 5 GB video) |
| Buckets | 10M |

Reads dominate **20:1**; large objects amplify egress bandwidth.

## Non-functional

- Durability **11 nines** — no silent loss of committed objects.
- GET p99 **&lt; 50ms** TTFB for hot objects; PUT p99 **&lt; 200ms** for objects &lt; 5 MB.
- Availability 99.99% per region.
- Strong read-after-write consistency for overwrite of same key.

## Your task

Whiteboard **45–55 minutes**:

1. Metadata vs data separation — what lives in an index DB vs on disk?
2. Upload path: small object vs multipart large object.
3. Download path: streaming, range requests, CDN integration.
4. Replication and erasure coding — how many copies, where?
5. Listing at trillion-object scale — index structure and pagination.
6. Delete and garbage collection of orphaned multipart parts.

## Stretch

- Cross-region replication (CRR) with eventual consistency.
- Lifecycle rules: transition to cold tier after 90 days.
- S3 Select / server-side filtering on object contents.
- Versioning, MFA delete, and legal hold.
