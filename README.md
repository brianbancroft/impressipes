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
| `mise run dev` | Run the SPA and API together |
| `mise run build` | Build everything into `dist/` |
| `mise run test` | Run all tests |
| `mise run lint` | `go vet` and TypeScript typecheck |

Individual projects work through Nx too, e.g. `pnpm nx dev web`, `pnpm nx test api`, `pnpm nx graph`.

## Adding projects

Each project has a `project.json` that declares its Nx targets. Give new apps and libraries the standard target names (`dev`, `build`, `test`, `lint`) so `run-many` picks them up. Go projects keep their own `go.mod` and `mise.toml`.

## Tracking

Issues, PRDs and designs live in Notion rather than in this repo: [Impressipies](https://app.notion.com/p/4076877dca3c800db24afb739385dc25).

- **Issues**: the issue tracker (status, priority, type, area, assignee), each optionally linked to a PRD.
- **PRDs**: product requirement docs, moving Draft, In Review, Approved, Shipped.
- **Designs**: Figma links, mockups and design decisions, one sub-page per feature or screen.
