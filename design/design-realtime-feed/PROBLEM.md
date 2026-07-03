# Design a realtime social feed

Design the **home feed** for a social app (Twitter/X or Instagram-style). Users follow accounts and see a reverse-chronological (or ranked) stream of posts.

## Functional requirements

1. **Post** — author creates text/media post; followers should see it in their feed.
2. **Home feed** — paginated list of posts from followed users (+ ads / recommendations later).
3. **Follow / unfollow** — updates whose posts appear.
4. **Like / comment counts** — eventually consistent is fine.
5. **Delete post** — remove from feeds (eventual OK).

## Scale (the fun part)

| Metric | Value |
|--------|-------|
| DAU | 100M |
| Posts / day | 200M (~2.3k writes/sec avg, 20k peak) |
| Feed reads / day | 10B (~115k reads/sec avg, 500k peak) |
| Avg follows | 200 |
| Celebrity accounts | up to 50M followers |

Reads dominate. A naive "join on read" dies immediately.

## Non-functional

- Feed p99 **&lt; 200ms** for first page.
- Post visible to followers within **&lt; 5s** (not strict real-time).
- Multi-region read latency matters for top markets.

## Your task

Whiteboard for **30–50 minutes**:

1. **Write path** — what happens when someone posts?
2. **Read path** — how does a user load page 1 of home feed?
3. **Celebrity problem** — 50M followers; fan-out on write can't work. What's the hybrid?
4. **Storage** — what tables / caches / logs, and how sharded?
5. Where **rate limiting** and **abuse** controls live.

Sketch data flow. Quantify rough QPS per component. Save ranking ML for "v2."

## Stretch

- How do you add **close friends** lists without doubling storage?
- Cold start for a **new user** with zero follows?
