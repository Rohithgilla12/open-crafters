# Design video upload and streaming

Design **YouTube / Netflix upload** (simplified): creators upload video; viewers stream with adaptive quality worldwide.

## Functional requirements

1. **Upload** — resumable upload for large files (GB-scale).
2. **Process** — transcode to multiple bitrates (360p, 720p, 1080p).
3. **Publish** — video becomes playable when minimum rendition ready.
4. **Playback** — HLS/DASH manifests; CDN serves segments.
5. **Metadata** — title, owner, duration, thumbnails.
6. **Delete** — remove from catalog + purge CDN (eventual).

## Scale

| Metric | Value |
|--------|-------|
| Uploads / day | 500k |
| Avg upload size | 2 GB |
| Views / day | 1B |
| Peak playback | 5M concurrent streams |
| Storage | exabytes over time |

## Non-functional

- Upload resume after network drop.
- Playback start **&lt; 2s** on good networks.
- Transcode lag: 1080p ready within **10 min** of upload complete (p95).

## Your task

Whiteboard **40–50 minutes**:

1. Multipart / resumable upload protocol.
2. Job pipeline: upload complete → transcode workers → publish.
3. Storage layout for raw + transcoded segments.
4. CDN + origin architecture for segment delivery.
5. How manifests point at renditions.

## Stretch

- Live streaming (RTMP ingest) — how it differs.
- DRM and signed URLs.
