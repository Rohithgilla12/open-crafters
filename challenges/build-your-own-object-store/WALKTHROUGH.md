# Walkthrough — Build your own object store

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint object-store` prints just the hint for your next stage;
`crafters walkthrough object-store --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Same newline-delimited JSON loop as every challenge: read line,
> decode, dispatch, respond, flush. `ping` returns `pong` — wire up transport
> once, then add object methods stage by stage.

**How it works:** The reference isolates RPC dispatch from TCP handling. Each
connection is independent. `--data-dir` is ignored until durability.

## put-get — Put and get

> **Hint:** `map[key]body` in memory. Etag is `sha256(body).hexdigest()` —
> lowercase hex, no prefix. `get` on a missing key returns `NOT_FOUND`, not an
> empty result.

**How it works:** `put` stores bytes and returns the etag. `get` loads the
object and returns `found`, `body`, `etag`, and `size` (byte length). The etag
is recomputed on read so callers can verify integrity.

## head — Head metadata

> **Hint:** Reuse the same lookup as `get`, but omit `body` from the result.
> Still error with `NOT_FOUND` when the key is absent.

**How it works:** `head` returns `found`, `etag`, and `size` only. This is how
real object stores let clients check existence and size before downloading.

## overwrite — Overwrite

> **Hint:** `put` is upsert — same key replaces the previous body and returns a
> fresh etag. No versioning required.

**How it works:** The reference overwrites the map entry in place. The new etag
reflects the new bytes; old data is gone.

## delete — Delete

> **Hint:** `delete` returns `deleted: true` when a key existed, `false`
> otherwise. After delete, `get`/`head` must return `NOT_FOUND`.

**How it works:** Remove the key from the object map. Idempotent delete of a
missing key is not an error.

## list — List by prefix

> **Hint:** Filter keys with `strings.HasPrefix` (or `key.startswith`), sort
> lexicographically, return `{"keys": [...]}`. Empty prefix lists everything.

**How it works:** A single sorted slice over matching keys. Lexicographic order
means `a/10` sorts before `a/2` — string order, not numeric.

## multipart — Multipart upload

> **Hint:** Track in-flight uploads by `upload_id` with a map of
> `part_number → body`. `complete_multipart` concatenates parts in ascending
> part number, checks each etag, stores at the original key, and removes the
> upload.

**How it works:** Part etags are SHA-256 of each part. The final etag is
SHA-256 of the concatenated assembly. Wrong order or etag → `INVALID_PART`.
Unknown upload → `NO_SUCH_UPLOAD`.

## durability — Survive a crash

> **Hint:** Snapshot the object map (and in-flight uploads) to `--data-dir`
> after every mutation — temp file + rename is enough for SIGKILL. Reload on
> startup.

**How it works:** The reference JSON-encodes objects as base64 in `state.json`.
Puts and deletes persist immediately; deletes must not resurrect after restart.

## gauntlet — The gauntlet

> **Hint:** Compose everything: put, get, head, delete, list, multipart, persist
> after every change. A crash mid-gauntlet must not lose acknowledged writes or
> resurrect deleted keys.

**How it works:** The gauntlet interleaves operations across rounds with
`SIGKILL` between them, then verifies the full key set. The same persist-on-
mutation path used in durability carries through.
