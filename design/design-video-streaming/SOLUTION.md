# Reference architecture — video streaming

## Upload path

```
Client ──▶ Upload API ──▶ Object store (multipart)
                │
                ▼
         videos DB (status=uploading)
                │
    complete ──▶ event ──▶ transcode queue
```

1. `POST /uploads` → `upload_id`, part URLs.
2. Client `PUT` parts with part numbers.
3. `POST /uploads/{id}/complete` → verify etags, mark `processing`.

## Transcode pipeline

```
Queue ──▶ Worker fleet (GPU)
            │
            ├─ download raw from object store
            ├─ ffmpeg → renditions (360/720/1080)
            ├─ upload segments + manifests
            └─ PATCH video status=published
```

Priority queue for premium creators. **Scheduler** retries failed jobs with backoff.

## Playback path

```
Player ──▶ CDN ──▶ (miss) Object store origin
   │
   └─ fetch .m3u8 manifest → pick bitrate → fetch .ts segments
```

Signed URLs or short-lived cookies on manifest. CDN **95%+ hit rate** on segments.

## Storage tiers

| Tier | Content |
|------|---------|
| Hot object store | Recent uploads + popular videos |
| Cold archive | Old raw masters (Glacier-class) |

Transcoded segments are small vs raw — keep all renditions hot longer.

## View counts

Beacon to **log** / queue; aggregate in batch (not per-segment DB write).

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Multipart blobs | `build-your-own-object-store` |
| Transcode jobs | `build-your-own-queue` |
| Retry / delayed publish | `build-your-own-scheduler` |

Upload is object store; everything after is async pipelines — classic media architecture.
