# Design a URL shortener

Design **bit.ly-style** link shortening: users submit long URLs, get short links, and visitors get HTTP redirects.

## Functional requirements

1. **Shorten** — given a long URL, return a short link (`https://go.example/Ab3xK9`).
2. **Redirect** — `GET /{code}` → `302` to the original URL.
3. **Custom alias** — optional vanity path (`/launch-party`) if available.
4. **Analytics** — click counts and rough geo/device (async OK).
5. **Expiration** — optional TTL; expired links return `410 Gone`.

## Scale

| Metric | Value |
|--------|-------|
| New URLs / day | 100M (~1.2k writes/sec avg, 10k peak) |
| Redirects / day | 10B (~115k reads/sec avg, 500k peak) |
| Total stored URLs | 500B over years (plan for growth) |
| Short code length | 6–8 URL-safe characters |

Reads dominate **100:1**.

## Non-functional

- Redirect p99 **&lt; 10ms** (excluding client RTT).
- Shorten p99 **&lt; 100ms**.
- 99.99% availability on redirect path.

## Your task

Whiteboard **30–40 minutes**:

1. Short code generation strategy and collision handling.
2. Storage schema for `code → long_url` (+ metadata).
3. Redirect hot path — cache layers, CDN, origin.
4. Analytics pipeline that doesn't block redirects.
5. Custom alias vs auto-generated code differences.

## Stretch

- Abuse: malware URLs, phishing takedowns.
- Preview mode (`?preview=1`) without counting a click.
