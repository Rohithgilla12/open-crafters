# Hints — realtime feed

## Fan-out strategies

- **Fan-out on write**: on post, push `post_id` into each follower's feed cache. Great for normal users; catastrophic for celebrities.
- **Fan-out on read**: at read time, merge recent posts from all followees. OK for small follow graphs; slow at 200 follows × heavy read QPS.
- **Hybrid**: write fan-out for users under a follower threshold; celebrities are **merged at read** from their own post list.

## Feed storage

Per-user feed is an ordered list of `post_id` (Redis ZSET, Cassandra wide row, or dedicated feed service). Keep **only recent N** (e.g. 1000) in hot storage; older pages from cold store.

## Post storage

Posts are immutable blobs: `post_id → {author, body, media_refs, created_at}`. Sharded by `post_id` or `author_id`.

## Social graph

`follows(follower_id → [followee_id])` — separate service, heavily cached. Unfollow = stop fan-out, lazy purge from feed cache.

## open-crafters tie-in

- **Append log** — post stream per user or global ingestion log
- **Bloom filter** — "have we already fan-out this post to this shard?"
- **Rate limiter** — post spam, follow churn
