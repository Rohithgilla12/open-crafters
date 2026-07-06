# Walkthrough — Build your own URL shortener

Meta-compose gateway — orchestrate reference services via env addresses.

## bind — Boot the stack

> **Hint:** Read `IDGEN_ADDR`, `BLOOM_ADDR`, `STORE_ADDR` from the environment. Your gateway only needs `ping` for this stage; the harness pings the references directly.

## shorten — Mint a short code

> **Hint:** `next_id` → bloom `add` on filter `codes` → object store `put` at `links/<code>`.

## resolve — Resolve a code

> **Hint:** bloom `contains` first; on probable hit, `get` `links/<code>` from object store.

## not-found — Unknown codes

> **Hint:** Return `found: false` when bloom says absent or store has no key.

## analytics — Record a click

> **Hint:** `put` to `clicks/<code>` in object store on `record_click`.

## bloom — Bloom membership

> **Hint:** `create` filter `codes` once (ignore `FILTER_EXISTS`); `add` every minted code.

## multi-url — Many URLs

> **Hint:** Same pipeline per URL; codes come from `next_id` so they stay unique.

## concurrent — Parallel shortens

> **Hint:** Mutex around shared client state if you cache connections; child services handle their own concurrency.

## gauntlet — The gauntlet

> **Hint:** Reuse shorten/resolve/record_click; no new RPCs.
