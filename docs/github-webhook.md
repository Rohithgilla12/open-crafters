# GitHub push → auto-grade

Push an open-crafters solution repository to GitHub and the hosted runner grades
it automatically, reporting results as a **GitHub Check** on the commit.

## How it works

1. GitHub sends `POST /v1/webhook/github` on push
2. Runner verifies the webhook signature, downloads the repo tarball
3. Finds `your_program.sh` + `.open-crafters/challenge`
4. Creates a GitHub Check named **open-crafters** (in progress)
5. Runs `crafters grade` in Docker (resumes the next unpassed stage)
6. Updates the check with success/failure and full logs

## Setup

### 1. GitHub personal access token

Create a PAT with **Contents: read** and **Checks: read & write**.

Save as `GITHUB_TOKEN` on the runner.

### 2. Webhook secret

```bash
openssl rand -hex 32
```

Save as `GITHUB_WEBHOOK_SECRET`.

### 3. Runner environment

```yaml
GITHUB_TOKEN: ${GITHUB_TOKEN}
GITHUB_WEBHOOK_SECRET: ${GITHUB_WEBHOOK_SECRET}
RUNNER_GITHUB_BRANCHES: default   # or * for all branches
```

### 4. GitHub webhook

**Repository → Settings → Webhooks → Add webhook**

| Field | Value |
|-------|-------|
| Payload URL | `https://runner.gilla.fun/v1/webhook/github` |
| Content type | `application/json` |
| Secret | your `GITHUB_WEBHOOK_SECRET` |
| Events | Just the **push** event |

### 5. Repository layout

Commit the scaffold from `crafters start`:

```
your_program.sh
main.py
.open-crafters/challenge       # e.g. build-your-own-wal
.open-crafters/progress.json   # optional; tracks passed stages
```

## Behaviour

| Push | Result |
|------|--------|
| Default branch, valid solution | Grade next stage + GitHub Check |
| Other branches | Ignored (unless `RUNNER_GITHUB_BRANCHES=*`) |
| No `.open-crafters/challenge` | Ignored silently |
| Branch delete | Ignored |

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| 401 on webhook | Secret mismatch |
| 503 GITHUB_TOKEN | Set token and redeploy runner |
| Check stuck in progress | `docker logs open-crafters-runner` |
| Tarball 404 | Token lacks repo access |
