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

## Database

The API stores everything in Postgres when `DATABASE_URL` is set, migrating the schema on boot. Without it the API runs on its in-memory household store and the list, recipe and sync endpoints answer `503`.

The Postgres 17 database is `impressipies-db` in the Impressipies project on Coolify. To connect the app:

1. Copy the database's **Postgres URL (internal)** from its Coolify page.
2. Add it to the `impressipes` application as `DATABASE_URL` (runtime only).
3. `mise run deploy`. The log shows `database migrated`, and, while no member exists, a `first run: open /invite/...` link for the admin account.

Live changes stream from `GET /api/events` (server-sent events, fed by Postgres `LISTEN/NOTIFY`). Behind a proxy, leave response buffering off for that path.

## Recipe photos

Uploads are resized to a 360px thumbnail and a 1280px large variant, encoded as lossless WebP, and written under `IMAGE_DIR` (`/data/images` in the image, `data/images` when unset). Files are only on that disk, so **mount a persistent volume at `/data`** in Coolify (Storages → Add → destination path `/data`), or photos vanish on every deploy. Back it up with the database.

Photos are served by `GET /api/images/{id}/{thumb|large}` to signed-in members only, with `Cache-Control: private, max-age=31536000, immutable` and an ETag.
