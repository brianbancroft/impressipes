# Deploying

The root `Dockerfile` builds the SPA, embeds it in the Go binary (`apps/api/internal/web/static`) and ships the binary on distroless. One container serves everything on `$PORT` (default 8080):

| Path | Served by |
| --- | --- |
| `/api/*` | The API, with the `/api` prefix stripped (same as the Vite dev proxy) |
| `/healthz` | Health check |
| anything else | The SPA, falling back to `index.html` for client-side routes |

```bash
docker build -t impressipies .
docker run --rm -p 8080:8080 impressipies
```

## Coolify

1. New application from this repo, branch `main`, build pack **Dockerfile**, port `8080`.
2. Leave Coolify's health check off. The image has no shell or curl, so its in-container check would always fail. The app answers `GET /healthz` for external monitors.
3. No auto-deploy: there is deliberately no GitHub webhook. Deploys are triggered from the command line (below).

The app is `impressipes` in the Impressipies project on `coolify_2`, built over SSH with a deploy key.

## Deploying from the command line

```bash
export COOLIFY_URL=https://your-coolify-host
export COOLIFY_TOKEN=...   # Coolify > Keys & Tokens > API tokens, "deploy" ability
mise run deploy            # or scripts/deploy; add --force for a no-cache rebuild
```

This pushes the current branch, then calls Coolify's deploy API for the app.
