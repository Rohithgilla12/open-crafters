# Durability Blog Series — Design Spec

**Date:** 2026-07-13  
**Site:** https://learn.gilla.fun/blog  
**Approach:** Match existing seed-post format (approved)

## Goals

1. Fill the durability roadmap gap after the existing WAL post.
2. Funnel readers into `crafters start` for queue → log → LSM → MVCC → object store.
3. Stay consistent with the four seeded posts (short concept + grader CTA).

## Non-goals

- Walkthrough / stage-by-stage teaching (separate content track)
- Medium deep-dives, diagrams, or multi-part series framing
- CMS, comments, newsletter
- New learn-app routes or SEO machinery (already shipped)

## Posts

| File | Title angle | Primary related | Also link |
|---|---|---|---|
| `at-least-once-queues.md` | Visibility timeouts & receipt fencing | `build-your-own-queue` | `/roadmaps/durability` |
| `append-only-logs.md` | Absolute offsets & consumer groups | `build-your-own-log` | `design-event-streaming` |
| `lsm-trees-explained.md` | Memtable → SSTables → compaction | `build-your-own-lsm` | `/roadmaps/durability` |
| `mvcc-snapshot-isolation.md` | Multi-version reads & first-committer-wins | `build-your-own-mvcc` | `design-payment-ledger` |
| `object-store-durability.md` | Etags, multipart, crash-safe blobs | `build-your-own-object-store` | `design-blob-storage` |

**Dates:** 2026-07-10 through 2026-07-14 (one day each, after WAL’s 2026-07-09) so newest-first listing stays sensible.

## Shared template

YAML frontmatter: `title`, `description`, `date`, `tags`, `related` (challenge/design slugs).

Body shape (~150–250 words):

1. Hook — the production contract in one sentence
2. 2–3 bullets on what the primitive owns
3. What open-crafters grades (behavioral / SIGKILL / byte-exact where true)
4. CTA: `crafters start <fuzzy>` + link to challenge and roadmap/design

## Delivery

- Create five files under `content/blog/` only
- Existing `go:embed content/blog` + `LoadBlogPosts()` pick them up automatically
- No learn template or route changes required
- Tests already assert `len(BlogPosts) >= 4`; optionally assert new slugs in sitemap

## Acceptance

1. `/blog` lists all five new posts
2. Each `/blog/{slug}` renders with title, related challenge links, and install CTA
3. `go test ./internal/learn/ -run SEO` passes
4. Posts match length/tone of `write-ahead-log-durability.md`
