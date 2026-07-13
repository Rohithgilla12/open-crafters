---
title: Object stores are durable blobs with etags
description: Put/get with SHA-256 etags, prefix listing, multipart upload, and crash-safe objects — the S3-shaped storage primitive.
date: 2026-07-14
tags: object-store, blob, durability
related: build-your-own-object-store, design-blob-storage
---

Blob storage looks simple until you define **durability and identity**. Clients put opaque bytes under a key, read them back bit-for-bit, and trust an etag to detect change — even after the process dies mid-write.

## The contract

- **Put / get** with content-addressed **SHA-256 etags**
- **Head** and **prefix list** without shipping full bodies
- **Multipart upload** assembles large objects from ordered parts
- Acknowledged objects survive crash; incomplete multipart work must not appear as complete

## What we grade

[Build your own object store](/challenges/build-your-own-object-store) exercises the full API under concurrent clients and repeated `SIGKILL`. If you ack a put, the bytes and etag must come back after restart.

## Start here

```sh
crafters start object-store
```

Capstone of the [durability roadmap](/roadmaps/durability), or whiteboard first: [Design blob storage](/design/design-blob-storage).
