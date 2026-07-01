# Learn app

The learn app is a lightweight HTTP server that serves the embedded challenge
catalog: stage instructions, protocol specs, and links to the hosted runner.

## Architecture

```
Browser  →  learn app (HTTPS via Cloudflare Tunnel)
                 ↓
           embedded challenges/ + goldmark HTML
```

| Component | Image | Role |
|-----------|-------|------|
| **learn** | `open-crafters-learn` | Challenge catalog, stage pages, JSON API |

Content is read from the same embedded `challenges/` tree as the `crafters` CLI.
Markdown is rendered with goldmark (GFM).

## Cloudflare Tunnel mapping

Add a **Public Hostname** in the Zero Trust dashboard (or your existing tunnel
config):

| Field | Value |
|-------|-------|
| **Subdomain** | `learn` |
| **Domain** | `gilla.fun` |
| **Service type** | HTTP |
| **URL** | `http://150.230.131.66:18081` |

Recommended hostname: **`https://learn.gilla.fun`**

**If `cloudflared` runs on the VPS host** with Docker port publishing, use
`deploy/vps-compose.yml` (port **18081** maps to container `:8081`).

## VPS deployment

```bash
# From your machine (repo root)
./scripts/vps-deploy.sh

# Or on the VPS directly
cd ~/open-crafters && ./scripts/vps-deploy.sh
```

Build the learn image manually:

```bash
# Build the image (from repo root)
docker build -f docker/learn/Dockerfile -t open-crafters-learn:latest .

# Start with the runner stack
docker compose -f deploy/vps-compose.yml up -d learn
```

The compose file publishes `18081:8081` on the host.

## Routes

| Route | Description |
|-------|-------------|
| `GET /` | Challenge catalog grouped by learning path (HTML) |
| `GET /paths/{slug}` | Single path — ordered challenge list |
| `GET /challenges/{slug}` | Challenge overview — stage list + protocol |
| `GET /challenges/{slug}/stages/{stage}` | Single stage with sidebar navigation |
| `GET /api/challenges` | JSON challenge list |
| `GET /api/paths` | JSON learning paths (`slug`, `name`, `description`, `challenges[]`) |
| `POST /api/submit` | Proxy solution zip to hosted runner (`Authorization: Bearer <token>`) |
| `GET /api/jobs/{id}` | Poll grading job status |
| `GET /learn.js` | Progress + submit client script |
| `GET /health` | Health check |
| `GET /style.css` | Stylesheet |

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `LEARN_LISTEN` | `:8081` | HTTP listen address |
| `LEARN_RUNNER_URL` | `https://runner.gilla.fun` | Hosted runner URL for browser submit proxy |
| `LEARN_MAX_ZIP_BYTES` | `10485760` | Max solution zip upload (10 MiB) |

## Browser submit

Each challenge page includes a **Submit to hosted runner** form. Zip your
solution directory (must contain `your_program.sh`), enter your runner token
(saved in browser localStorage), and grade remotely — the learn app proxies to
the runner API (no CORS issues).

Progress (stages read / passed) is tracked in **localStorage** and shown on
challenge cards and stage lists. Sync with the CLI:

1. Locally: `cd my-wal && crafters progress export > progress.json`
2. On this site: use **Export / Import progress.json** on the home page
3. Back to CLI: `crafters progress import progress.json my-wal`

Use `crafters progress export --all` from a parent directory to merge every
solution's `.open-crafters/progress.json` into one file.

## Local development

```bash
go run ./cmd/learn
# open http://localhost:8081
```

## Related

- [Hosted runner](hosted-runner.md) — `https://runner.gilla.fun`
- Install: `curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh`
