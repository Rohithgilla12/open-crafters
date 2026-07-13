# Durability Blog Series Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add five seed-format blog posts covering the durability roadmap after WAL (queue, log, LSM, MVCC, object store).

**Architecture:** Markdown files under `content/blog/` with YAML frontmatter; existing `go:embed` and `LoadBlogPosts()` surface them on learn.gilla.fun with no code changes.

**Tech Stack:** Markdown + YAML frontmatter; Go learn catalog (already embedded).

## Global Constraints

- Match seed length/tone of `content/blog/write-ahead-log-durability.md` (~150–250 words)
- Frontmatter keys: `title`, `description`, `date`, `tags`, `related`
- Dates: 2026-07-10 … 2026-07-14
- No CMS, walkthrough detail, or learn-app route changes
- Related slugs must exist in the catalog

---

### Task 1: Queue + Log posts

**Files:**
- Create: `content/blog/at-least-once-queues.md`
- Create: `content/blog/append-only-logs.md`
- Test: `internal/learn/seo_routes_test.go` (existing suite)

**Interfaces:**
- Consumes: existing `LoadBlogPosts()` / `/blog/{slug}` routes
- Produces: slugs `at-least-once-queues`, `append-only-logs`

- [ ] **Step 1: Write `at-least-once-queues.md`**

```markdown
---
title: At-least-once delivery is a visibility timeout
description: Persist before ack, fence with receipts, and redeliver when leases expire — how durable message queues actually work.
date: 2026-07-10
tags: queue, messaging, durability
related: build-your-own-queue, design-distributed-scheduler
---

A durable queue is not “FIFO in memory.” It is a promise: **every message is delivered at least once**, even if consumers crash mid-processing.

## The contract

- **Persist** the message before acknowledging the producer
- Hand out work with a **visibility timeout** (lease) so only one consumer sees it at a time
- **Fence** completions with receipts so stale consumers cannot ack after a redelivery
- Route poison messages to a **dead-letter** path after too many failures

## What we grade

[Build your own message queue](/challenges/build-your-own-queue) talks to your broker over TCP and `SIGKILL`s it mid-flight. Unacked work must reappear; acked work must not. On-disk format is yours — behavior is not.

## Start here

```sh
crafters start queue
```

Then continue the [durability roadmap](/roadmaps/durability): WAL → queue → log → LSM → MVCC → object store.
```

- [ ] **Step 2: Write `append-only-logs.md`**

```markdown
---
title: Append-only logs keep offsets absolute
description: Topics, permanent offsets, consumer-group commits, and retention that never renumbers — the Kafka-style log contract.
date: 2026-07-11
tags: log, streaming, kafka
related: build-your-own-log, design-event-streaming
---

Event streams are not queues you drain. They are **append-only logs**: producers write, consumers replay from an offset, and retention deletes old bytes without renumbering history.

## The contract

- Each record gets a **monotonic, absolute offset** (starting at 0) that never moves
- Reads are **replayable** — consuming does not delete
- **Consumer groups** store committed offsets so workers resume after crashes
- **Retention / truncate** may drop a prefix, but surviving offsets stay the same numbers

## What we grade

[Build your own log](/challenges/build-your-own-log) checks append/read, multi-topic isolation, consumer-group commits, retention, and crash survival. The harness restarts your process with the same `--data-dir` and expects the same offsets back.

## Start here

```sh
crafters start log
```

Whiteboard the bigger picture: [Design an event streaming platform](/design/design-event-streaming).
```

- [ ] **Step 3: Verify posts load**

Run: `go test ./internal/learn/ -run 'TestSEO|TestBlog' -count=1`
Expected: PASS (catalog includes ≥4 posts; new slugs served)

- [ ] **Step 4: Commit** (when asked)

```bash
git add content/blog/at-least-once-queues.md content/blog/append-only-logs.md
git commit -m "Add queue and log durability blog posts."
```

---

### Task 2: LSM + MVCC posts

**Files:**
- Create: `content/blog/lsm-trees-explained.md`
- Create: `content/blog/mvcc-snapshot-isolation.md`
- Test: `internal/learn/seo_routes_test.go`

**Interfaces:**
- Produces: slugs `lsm-trees-explained`, `mvcc-snapshot-isolation`

- [ ] **Step 1: Write `lsm-trees-explained.md`**

```markdown
---
title: LSM-trees turn random writes into sequential I/O
description: Memtables, byte-exact SSTables, range scans, compaction, and tombstones — how RocksDB-style engines stay durable under crash.
date: 2026-07-12
tags: lsm, storage, sstable
related: build-your-own-lsm, design-distributed-kv
---

Databases that absorb heavy write traffic rarely update pages in place. They buffer in a **memtable**, flush sorted **SSTables**, and merge in the background. That lifecycle is the log-structured merge-tree.

## The contract

- Writes hit an in-memory memtable first, then flush to immutable SST files
- **Latest value wins** across overlapping files after compaction
- **Tombstones** hide deleted keys until compaction can drop them
- After a crash, reopening the SST set reconstructs exactly what you acknowledged

## What we grade

[Build your own LSM-tree](/challenges/build-your-own-lsm) grades both the wire API and a **byte-exact SST format** the tester parses offline. SIGKILL gauntlets check that flushed state survives and matches what you served.

## Start here

```sh
crafters start lsm
```

On the [durability roadmap](/roadmaps/durability) this sits between the append log and MVCC.
```

- [ ] **Step 2: Write `mvcc-snapshot-isolation.md`**

```markdown
---
title: Snapshot isolation is multi-version reads
description: Frozen snapshots, buffered writes, and first-committer-wins conflicts — how transactional KV stores stay consistent under concurrency.
date: 2026-07-13
tags: mvcc, transactions, isolation
related: build-your-own-mvcc, design-payment-ledger
---

A transactional store does not hand every reader the latest write. Under **snapshot isolation**, a transaction freezes a consistent view at begin time, buffers its own writes, and only publishes them if no conflicting commit landed first.

## The contract

- Reads see a **snapshot** of committed data as of `begin`, plus the txn’s own writes
- Concurrent writers that touch the same key: **first committer wins**; the loser aborts
- Uncommitted work is invisible to others and disappears on rollback or crash
- Only **committed** transactions survive `SIGKILL`

## What we grade

[Build your own MVCC](/challenges/build-your-own-mvcc) runs overlapping clients, checks snapshot reads, atomic multi-key commits, conflict detection, deletes, and crash durability. On-disk layout is yours — observable isolation is not.

## Start here

```sh
crafters start mvcc
```

See it in an interview framing: [Design a payment ledger](/design/design-payment-ledger).
```

- [ ] **Step 3: Run learn SEO tests**

Run: `go test ./internal/learn/ -run SEO -count=1`
Expected: PASS

---

### Task 3: Object store post + sitemap coverage

**Files:**
- Create: `content/blog/object-store-durability.md`
- Modify: `internal/learn/seo_routes_test.go` — assert one new durability slug in sitemap
- Test: `internal/learn/seo_routes_test.go`

**Interfaces:**
- Produces: slug `object-store-durability`; stronger test floor for durability series

- [ ] **Step 1: Write `object-store-durability.md`**

```markdown
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
```

- [ ] **Step 2: Strengthen SEO test**

In `internal/learn/seo_routes_test.go`, change the blog post floor and add the object-store URL to the sitemap expected list:

```go
if len(catalog.BlogPosts) < 9 {
	t.Fatalf("expected >= 9 blog posts, got %d", len(catalog.BlogPosts))
}
```

And include `"https://learn.gilla.fun/blog/object-store-durability"` among expected sitemap URLs (alongside the existing WAL URL).

- [ ] **Step 3: Run full learn package tests**

Run: `go test ./internal/learn/ -count=1`
Expected: PASS

- [ ] **Step 4: Commit all five posts + test + docs** (when asked)
