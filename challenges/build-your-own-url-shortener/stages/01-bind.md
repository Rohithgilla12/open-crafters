# Stage 1: Boot the stack

The harness starts three **reference** services and your gateway. Implement `ping` on the gateway; the tester also pings id-generator, bloom-filter, and object-store directly.

Read `IDGEN_ADDR`, `BLOOM_ADDR`, `STORE_ADDR` from the environment.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Read `IDGEN_ADDR`, `BLOOM_ADDR`, `STORE_ADDR` from the environment. Your gateway only needs `ping` for this stage; the harness pings the references directly.

Or run: <code>crafters hint url-shortener --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
