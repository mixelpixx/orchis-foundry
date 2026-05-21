# Orchis Foundry

A modern, self-hosted code platform. Calm, minimal UI. Apple's 3-step rule, enforced.

This repository contains:

- **`web/`** — the frontend prototype (HTML + React via Babel-in-browser). Currently lives at the root as `index.html` + `src/` + `tweaks-panel.jsx`. Treat it as the spec.
- **`CLAUDE.md`** — the backend implementation guide. Read this first if you're picking up the backend work.

## Quick tour of the frontend

Open `index.html` in a modern browser. No build step.

- **Repo view** (`src/views/repo.jsx`) — file tree + code + readme + IDE-style split panel
- **PR review** (`src/views/pr.jsx`) — inline-comment diff, review submit bar
- **Dashboard** (`src/views/dashboard.jsx`) — inbox, pinned repos, activity
- **Developer settings** (`src/views/devsettings.jsx`) — PATs, SSH keys, webhooks, all top-level
- **Command palette** (`src/palette.jsx`) — `⌘K` / `Ctrl+K`. The 3-step accelerator.

### Things to try
- `⌘K` → type `token` → Enter → name + scopes + create. Three steps to a PAT.
- Inside a repo, click `PRs` in the header to dock a PR list beside the code. `⌘\` toggles.
- In a diff, hover a line to reveal the `+` button for an inline comment.

### Tweaks
Open the in-app Tweaks panel to swap theme mode, accent (Spring / Cobalt / Ember / Violet), density, and font.

## Backend

Not built yet. See **`CLAUDE.md`** for the full stack decision, schema, endpoint contract, and implementation order. TL;DR:

- **Go** + chi + Postgres + sqlc, embedded SSH server, SSE for realtime
- **One binary**, one config file, one database
- Implementation order in §11; stop & demo after the PAT flow (step 4) — that's the proof the design works

## License

MIT.
