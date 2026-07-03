# Design a rate-limited API gateway

Design the **front door** for a public API platform (Stripe / Twilio style): developers call your HTTPS API with API keys; you authenticate, rate-limit, route to internal microservices, and return responses.

## Functional requirements

1. **Terminate TLS** and validate API keys / OAuth tokens.
2. **Route** `GET /v1/users/{id}` → correct backend service.
3. **Rate limit** per API key, per endpoint, and per tenant tier (free vs paid).
4. **Transform** — optional request/response mapping, header injection.
5. **Observability** — request IDs, latency metrics, audit log.
6. **Health-aware routing** — skip unhealthy backends.

## Scale

| Metric | Value |
|--------|-------|
| Requests / sec | 200k peak |
| API keys | 5M |
| Backend services | 80 |
| Regions | 3 |

## Non-functional

- Gateway add latency p99 **&lt; 5ms** (excluding backend).
- Rate limit decisions must be **correct under concurrency** (no 2× burst from races).
- Zero cross-tenant data leaks via mis-routing.

## Your task

Whiteboard **35–45 minutes**:

1. Edge vs regional vs central gateway tiers.
2. Rate limit state — where stored, how synchronized.
3. Auth validation hot path (cache JWT introspection?).
4. Service discovery and load balancing.
5. What you cache (and TTL) vs pass-through.

## Stretch

- GraphQL federation through the gateway.
- WAF / bot detection integration.
