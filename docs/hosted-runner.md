# Hosted runner

The hosted runner grades learner submissions in isolated Docker containers on a
VPS. Learners use `crafters submit`; the runner unpacks the zip and runs the
same `crafters grade` harness used locally.

## Architecture

```
crafters submit  →  runner API (HTTPS via Cloudflare Tunnel)
                         ↓
                   docker run (sandbox)
                         ↓
                   crafters grade --program /work/your_program.sh
```

| Component | Image | Role |
|-----------|-------|------|
| **runner** | `open-crafters-runner` | HTTP API, job queue, talks to Docker socket |
| **grade job** | `open-crafters-grade` | Ephemeral container per submission; has `crafters`, Python, Go, Bun |

Each grade job runs with `--network none`, CPU/RAM limits, and a 15-minute
timeout. Loopback TCP between the harness and the learner process still works.

## Cloudflare Tunnel mapping

Add a **Public Hostname** in the Zero Trust dashboard (or your existing tunnel
config):

| Field | Value |
|-------|-------|
| **Subdomain** | `runner` |
| **Domain** | `gilla.fun` |
| **Service type** | HTTP |
| **URL** | `http://localhost:8080` if `cloudflared` runs on the VPS host and the Portainer stack publishes `127.0.0.1:8080` |
| | `http://open-crafters-runner:8080` if `cloudflared` is a container on the `open-crafters` Docker network |

Recommended hostname: **`https://runner.gilla.fun`**

Optional hardening in Cloudflare:

- **Access** policy (service token or email allowlist) in front of the API
- **Rate limiting** on `POST /v1/grade`

The runner still requires `Authorization: Bearer <RUNNER_TOKEN>` on every API call.

## VPS setup (Portainer)

### 1. Clone and build images

```bash
git clone https://github.com/Rohithgilla12/open-crafters.git
cd open-crafters
git checkout feat/hosted-runner   # until merged to main

docker build -t open-crafters-grade:latest -f docker/grade/Dockerfile .
docker build -t open-crafters-runner:latest -f docker/runner/Dockerfile .
```

### 2. Prepare host job directory

The runner spawns grade containers through the Docker socket; job files must
live on a **host bind mount** (not a Docker named volume):

```bash
sudo mkdir -p /var/lib/open-crafters/jobs
sudo chown "$USER:$USER" /var/lib/open-crafters/jobs
```

### 3. Generate a token

```bash
openssl rand -hex 32
```

Save this as `RUNNER_TOKEN` in the Portainer stack environment.

### 4. Deploy the stack

In Portainer: **Stacks → Add stack** → paste `deploy/portainer-stack.yml` → set
`RUNNER_TOKEN` → deploy.

### 5. Smoke test

```bash
curl -s https://runner.gilla.fun/health
# {"status":"ok"}

curl -s -H "Authorization: Bearer $RUNNER_TOKEN" \
  https://runner.gilla.fun/v1/jobs/00000000000000000000000000000000
# 404 not found (expected)
```

## Learner usage

```bash
export CRAFTERS_RUNNER_URL=https://runner.gilla.fun
export CRAFTERS_RUNNER_TOKEN=<same secret as RUNNER_TOKEN>

crafters start wal
cd my-wal
# ... implement stages ...
crafters submit              # grades the next unpassed stage
crafters submit --all        # full challenge
crafters submit --no-wait    # returns job id immediately
```

## API

### `GET /health`

No auth. Returns `{"status":"ok"}`.

### `POST /v1/grade`

`multipart/form-data`:

| Field | Required | Description |
|-------|----------|-------------|
| `challenge` | yes | Full slug or passed through as-is (e.g. `build-your-own-wal`) |
| `file` | yes | Zip of the solution directory (must contain `your_program.sh`) |
| `all` | no | `true` to run every stage |
| `stage` | no | Run up to and including this stage slug |

Headers: `Authorization: Bearer <token>` or `X-Crafters-Token: <token>`

Returns `202` with a job object. Poll `GET /v1/jobs/{id}` until `status` is
`passed`, `failed`, or `error`.

### `GET /v1/jobs/{id}`

Returns the job including `log` (full `crafters grade` output).

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RUNNER_TOKEN` | — | **Required.** API bearer token |
| `RUNNER_LISTEN` | `:8080` | Listen address |
| `RUNNER_GRADE_IMAGE` | `open-crafters-grade:latest` | Sandbox image |
| `RUNNER_MAX_CONCURRENT` | `2` | Max parallel grade jobs |
| `RUNNER_JOB_TIMEOUT` | `15m` | Per-job wall clock limit |
| `RUNNER_WORK_DIR` | `/var/lib/open-crafters/jobs` | Temp extraction root |
| `RUNNER_MAX_ZIP_BYTES` | `10485760` | Max upload size (10 MiB) |

## Updating

After pulling new code:

```bash
docker build -t open-crafters-grade:latest -f docker/grade/Dockerfile .
docker build -t open-crafters-runner:latest -f docker/runner/Dockerfile .
```

Restart the runner stack in Portainer. Grade jobs pick up the new grade image
on the next `docker run`.

## Security notes

- The runner container mounts `/var/run/docker.sock` — keep the API private
  (tunnel + token; do not publish port 8080 publicly).
- Job workspaces use a **host bind mount** at `/var/lib/open-crafters/jobs` so
  grade containers can mount submission files. A Docker named volume will not
  work because paths passed to `docker run -v` are resolved on the host.
- Submissions are untrusted; never run `your_program.sh` on the host directly.
- Grade containers have no network; solutions using external package downloads
  at runtime may need vendoring (starters use stdlib / Bun with no deps).
