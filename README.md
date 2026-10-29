# Impressipies

An [Nx](https://nx.dev) monorepo containing a React single-page app and a Go API. Tooling versions are managed with [mise](https://mise.jdx.dev).

## Layout

```
apps/
  web/   Vite + React + TypeScript SPA (dev server on :4200, proxies /api -> :8080)
  api/   Go HTTP API (net/http, listens on :8080)
mise.toml   Pinned tool versions (node, pnpm, go) and root tasks
nx.json     Nx workspace config and cached target defaults
```

## Prerequisites

Install [mise](https://mise.jdx.dev/getting-started.html), then from the repo root:

```bash
mise install
mise run install
```

## Common tasks

| Command | What it does |
| --- | --- |
| `mise run dev` | Run Mailpit, the SPA and API together |
| `mise run up` | Start Postgres (Docker, :5433), Mailpit, API and SPA in the background; seeds the DB unless the seed is already in it (logs in `.run/`) |
| `mise run stop` | Kill the stack's processes and anything listening on :1025, :8025, :8080, :4200; stop Postgres (data kept) |
| `mise run down` | Undo `up`: `stop`, then remove the Postgres container and volume, `.run/` and `.data/` |
| `mise run reset` | Drop the local database, re-migrate and re-seed (restarts the API if running) |
| `mise run nuke` | Delete every saved image (`.data/images` and the `recipe_images` rows) |
| `mise run mailpit` | Run Mailpit alone (SMTP `:1025`, inbox UI http://localhost:8025) |
| `mise run build` | Build everything into `dist/` |
| `mise run test` | Run all tests |
| `mise run lint` | `go vet` and TypeScript typecheck |

### Local email

`mise run dev` starts [Mailpit](https://mailpit.axllent.org) (install with `brew install mailpit`) and points the API at it via `SMTP_ADDR`. On a first run with no members, the API emails the bootstrap admin invite to `admin@impressipes.com` (override with `BOOTSTRAP_ADMIN_EMAIL`; links use `APP_URL`, default `http://localhost:4200`). Read it in the Mailpit inbox at http://localhost:8025. In production set `RESEND_API_KEY` and `MAIL_FROM` (a verified Resend sender) and the same email goes out via Resend. With neither `SMTP_ADDR` nor `RESEND_API_KEY`, no email is sent and the link is only logged.

Individual projects work through Nx too, e.g. `pnpm nx dev web`, `pnpm nx test api`, `pnpm nx graph`.

## Adding projects

Each project has a `project.json` that declares its Nx targets. Give new apps and libraries the standard target names (`dev`, `build`, `test`, `lint`) so `run-many` picks them up. Go projects keep their own `go.mod` and `mise.toml`.

## Tracking

Issues, PRDs and designs live in Notion rather than in this repo: [Impressipies](https://app.notion.com/p/4076877dca3c800db24afb739385dc25).

- **Issues**: the issue tracker (status, priority, type, area, assignee), each optionally linked to a PRD.
- **PRDs**: product requirement docs, moving Draft, In Review, Approved, Shipped.
- **Designs**: Figma links, mockups and design decisions, one sub-page per feature or screen.
