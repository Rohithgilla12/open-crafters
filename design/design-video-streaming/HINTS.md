# Hints — video streaming

## Resumable upload

**Multipart upload** to object storage — your **object store** challenge (initiate, upload parts, complete, etag).

Client tracks completed part numbers; retry failed parts only.

## Processing pipeline

Upload complete event → **queue** message → transcode workers pull jobs → write renditions back to object store → update metadata `status=ready`.

## Segment storage

HLS: directory per video `/vid123/720p/segment_0001.ts`. Manifest `.m3u8` lists segments.

CDN caches segments aggressively; origin is object store.

## Metadata vs bytes

`videos` DB row: `{id, owner, title, status, manifest_urls}`. Bytes live only in object store.

## open-crafters tie-in

- **Object store** — multipart blobs + etags
- **Queue** — transcode job delivery
- **Scheduler** — delayed publish, retry failed transcodes
