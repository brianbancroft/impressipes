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

## Email sign-in

Members sign in with an email and password, or with a link and six-digit code that is emailed to them (the link also covers new phones and forgotten passwords). Email confirmation uses the same mail. Set these on the `impressipes` application in Coolify:

| Variable | Purpose |
| --- | --- |
| `RESEND_API_KEY` | Sends the mail through Resend (production). Locally `SMTP_ADDR` points at Mailpit instead. |
| `MAIL_FROM` | Sender, e.g. `Impressipes <noreply@your-domain>`. Required with `RESEND_API_KEY`; the domain must be verified in Resend. |
| `APP_URL` | Public origin the emailed links point at: `https://impressipes.bancroft.io`. |
| `APPLE_TEAM_ID` | Optional. Serves `/.well-known/apple-app-site-association` so tapping the emailed link opens the iOS app (the app declares `applinks:impressipes.bancroft.io`). Without it the link opens the web app and the code still works in the app. |

Without a mail sender the API logs each link and code instead of sending it, which is for local development only. Links and codes work once, expire after 15 minutes, and a code locks after five wrong tries. Requests are rate limited per address and per client IP.

Passwords are stored as argon2id hashes and need at least 10 characters. A member's email only becomes usable for sign-in once confirmed.

## Recipe photos

Uploads are resized to a 360px thumbnail and a 1280px large variant, encoded as lossless WebP, and written under `IMAGE_DIR` (`/data/images` in the image, `data/images` when unset). Files are only on that disk, so **mount a persistent volume at `/data`** in Coolify (Storages → Add → destination path `/data`), or photos vanish on every deploy. Back it up with the database.

Photos are served by `GET /api/images/{id}/{thumb|large}` to signed-in members only, with `Cache-Control: private, max-age=31536000, immutable` and an ETag.
