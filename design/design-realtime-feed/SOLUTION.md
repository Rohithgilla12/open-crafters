# Reference architecture — realtime feed

## Write path (normal user, 500 followers)

```
Post API → validate → store post blob → publish event
                │
                ▼
        Fan-out workers (async)
                │
    for each follower_id (batched):
        LPUSH feed:{follower_id} post_id
        trim to max length
```

Post API returns fast; fan-out is **async** via queue / log consumers.

## Write path (celebrity, 50M followers)

Skip per-follower fan-out. Instead:

1. Store post in `posts_by_author:{celebrity_id}` (sorted set).
2. Mark author as `celebrity` in graph service.
3. Followers of celebrities **merge at read** (see below).

Optional: fan-out to a **sample** of active followers only.

## Read path (hybrid merge)

```
GET feed page:
  1. Fetch precomputed feed slice from cache (fan-out writes)
  2. Fetch list of followed celebrities
  3. For each celebrity (parallel, capped): top K recent posts
  4. Merge-sort by timestamp, dedupe, paginate
```

Cache key: `feed:{user_id}` — ZSET of `(timestamp, post_id)`.

## Component diagram

```
┌──────────┐     ┌─────────────┐     ┌──────────────┐
│ Post API │────▶│ Post store  │     │ Graph service│
└────┬─────┘     └─────────────┘     └──────┬───────┘
     │ event                                  │
     ▼                                        │
┌─────────────┐         ┌─────────────────────┘
│ Fan-out     │         │
│ workers     │         ▼
└──────┬──────┘   ┌─────────────┐
       │          │ Feed cache  │◀── read API
       └─────────▶│ (per user)  │
                  └─────────────┘
```

## Sharding

- **Posts**: shard by `post_id` hash.
- **Feeds**: shard by `user_id` — co-locate with graph cache where possible.
- **Fan-out queue**: partition by `follower_id` for even load.

## Consistency

Eventual is fine: counts lag seconds; deleted posts removed by tombstone events fanning through same pipeline.

## Rate limiting

- Post: per-user token bucket (your **rate limiter** challenge).
- Follow: stricter limits + captcha triggers.
- Read: CDN edge cache for public profiles; authenticated home feed bypasses CDN.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Post ingestion log | `build-your-own-log` |
| Fan-out work queue | `build-your-own-queue` |
| Abuse throttling | `build-your-own-rate-limiter` |

The feed problem is mostly **architecture around** primitives you've built — the hot insight is hybrid fan-out, not a novel data structure.
