# Stage 3: Resolve a code

`resolve` with `{"code": "..."}` → `{"found": true, "url": "..."}` or `{"found": false}`.

Check bloom `contains`, then object store `get` on `links/<code>`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** bloom `contains` first; on probable hit, `get` `links/<code>` from object store.

Or run: <code>crafters hint url-shortener --stage resolve</code>
</details>
<!-- /crafters-stage-hint -->
