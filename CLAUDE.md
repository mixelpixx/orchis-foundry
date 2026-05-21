# Orchis Foundry — Backend Implementation Guide

> This document is the contract between the existing **Orchis Foundry** frontend prototype (`index.html` + `src/`) and the backend you are about to build. Read it end-to-end before writing code. The frontend is intentionally opinionated; the backend should be **boring, fast, and self-hostable**.

---

## 0. Product north star

A modern, self-hosted code platform. The UX rule is **Apple's 3-step rule**: every meaningful action reachable in ≤ 3 steps. The frontend honors this through a `⌘K` command palette, an IDE-style split view, and a flat developer-settings surface (PATs, SSH keys, webhooks all top-level).

The backend must support that promise. That means:
- **Fast endpoints** (target p95 < 100 ms for all reads on a warm cache).
- **Atomic actions** — `POST /tokens` returns the secret once, no multi-step wizards.
- **Real-time** where it matters (PR activity, check runs).
- **One binary, one config file, one database** to self-host.

---

## 1. Stack

| Layer | Choice | Why |
|---|---|---|
| **Language** | **Go 1.22+** | Single static binary, mature git ecosystem (go-git, gitea precedent), great stdlib HTTP, low ops burden. |
| **HTTP router** | `chi` (`github.com/go-chi/chi/v5`) | Lightweight, idiomatic, middleware-first. |
| **Database** | **PostgreSQL 15+** (prod), **SQLite** (dev / single-user) | Same SQL via `sqlc`; choice at config time. |
| **Migrations** | `goose` | Plain SQL, reversible, embed-friendly. |
| **Queries** | `sqlc` | Type-safe Go from SQL. No ORM. |
| **Git core** | `go-git` for in-process reads; **shell out to `git`** for writes, clones, packfiles, GC | Best of both — fast reads, correct & maintained writes. |
| **Auth** | Argon2id passwords + OIDC providers (GitHub, GitLab, generic) + SSH key auth for git | Industry default; OIDC keeps it simple. |
| **Session** | Signed, HttpOnly, SameSite=Lax cookies (PASETO v4 local tokens) | PASETO over JWT — no algorithm-confusion footguns. |
| **API tokens (PATs)** | Random 32-byte tokens, prefixed `orc_pat_`, stored as Argon2id hashes | Show once, hash at rest, scope-checked per request. |
| **Realtime** | **Server-Sent Events** (`/v1/stream`) | One-way, HTTP/2-friendly, no WebSocket complexity. |
| **Object storage** | Local filesystem by default; S3-compatible via `gocloud.dev/blob` | For LFS, attachments, avatars. |
| **Search** | Postgres full-text for issues/PRs; **`ripgrep` shell-out** for code search inside repos | Don't reinvent. |
| **SSH server** | `gliderlabs/ssh` embedded in the binary, port 2222 by default | Speaks the git wire protocol; reads keys from DB. |
| **Container** | `docker compose up` for self-host: app + Postgres + (optional) MinIO | Smallest possible "git going" experience. |
| **Logs** | `log/slog` JSON to stdout | Run-anywhere; ship to whatever. |
| **Metrics** | Prometheus `/metrics` | Standard. |
| **Frontend** | The existing `index.html` + `src/*.jsx`, served as static files. **Do not rewrite the frontend.** | It is the contract. |

### Hard "do nots"
- ❌ No microservices. One binary.
- ❌ No GraphQL. REST + SSE.
- ❌ No CRDTs, no event-sourced anything. Boring SQL.
- ❌ No JWT-with-`alg: none`. Use PASETO.
- ❌ No ORM. `sqlc` only.
- ❌ No frontend rewrite. The JSX is the spec.

---

## 2. Repository layout

```
orchis-foundry/
├── cmd/
│   ├── orchis/              # main binary: HTTP + SSH server
│   └── orchis-migrate/      # standalone migration runner
├── internal/
│   ├── api/                 # HTTP handlers, one file per resource
│   │   ├── repos.go
│   │   ├── pulls.go
│   │   ├── tokens.go
│   │   ├── ssh_keys.go
│   │   ├── webhooks.go
│   │   ├── search.go
│   │   ├── stream.go        # SSE
│   │   └── middleware.go
│   ├── auth/                # password, OIDC, sessions, PAT validation
│   ├── git/                 # go-git + shell wrappers
│   ├── storage/             # blob abstraction
│   ├── db/                  # sqlc-generated code
│   │   ├── queries/         # .sql files
│   │   └── migrations/      # .sql files (goose)
│   ├── sshd/                # embedded SSH server for git
│   ├── webhook/             # outbound delivery worker
│   ├── search/              # FTS + ripgrep wrapper
│   └── config/              # viper-free, just env + yaml
├── web/                     # frontend lives here, served at /
│   ├── index.html
│   ├── src/...
│   └── tweaks-panel.jsx
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml
├── docs/
│   ├── api.md               # OpenAPI 3.1 source
│   └── deploy.md
├── CLAUDE.md                # this file
├── README.md
├── go.mod
└── Makefile
```

---

## 3. The frontend contract — endpoints you must implement

The JSX files in `src/views/` are the source of truth. Every piece of data the UI references must have a real endpoint. Below is the full required set, grouped by view. **Paths are versioned under `/v1`.**

### 3.1 Auth & session

```
POST   /v1/auth/login            { handle, password }              → 204 + cookie
POST   /v1/auth/logout                                             → 204
GET    /v1/auth/oidc/:provider   redirect to provider
GET    /v1/auth/oidc/callback    code, state                       → 302 to /
GET    /v1/me                                                      → User
PATCH  /v1/me                    { name?, email?, ... }            → User
```

### 3.2 Repositories — drives `RepoView` (`src/views/repo.jsx`)

```
GET    /v1/repos                            ?org=&q=&pinned=       → [Repo]
POST   /v1/repos                            { org, name, ... }     → Repo
GET    /v1/repos/:org/:name                                        → Repo
PATCH  /v1/repos/:org/:name                 { description, ... }   → Repo
DELETE /v1/repos/:org/:name                                        → 204

# Browsing — see FILE_TREE & ROUTER_RS in src/data.jsx for shape
GET    /v1/repos/:org/:name/tree            ?ref=&path=            → [Node]
GET    /v1/repos/:org/:name/blob            ?ref=&path=            → { content, lang, size, lines }
GET    /v1/repos/:org/:name/raw             ?ref=&path=            → raw bytes
GET    /v1/repos/:org/:name/readme          ?ref=                  → { content, format }
GET    /v1/repos/:org/:name/branches                               → [Branch]
GET    /v1/repos/:org/:name/commits         ?ref=&path=&limit=     → [Commit]
GET    /v1/repos/:org/:name/commits/:sha                           → CommitDetail
```

**Pin/star** (sidebar uses `r.pinned`):
```
PUT    /v1/me/pinned/:org/:name
DELETE /v1/me/pinned/:org/:name
PUT    /v1/me/stars/:org/:name
DELETE /v1/me/stars/:org/:name
```

### 3.3 Pull requests — drives `PRView` & `PRsView` (`src/views/pr.jsx`)

```
GET    /v1/pulls                            ?filter=review-requested|yours|mentioned|all
                                                                   → [PullSummary]
GET    /v1/repos/:org/:name/pulls           ?state=open|closed|all → [PullSummary]
POST   /v1/repos/:org/:name/pulls           { title, head, base, body }
                                                                   → Pull
GET    /v1/repos/:org/:name/pulls/:num                             → PullDetail

# Diff — see PR_DIFF_FILES in src/data.jsx
GET    /v1/repos/:org/:name/pulls/:num/files                       → [FileDiff]
GET    /v1/repos/:org/:name/pulls/:num/commits                     → [Commit]
GET    /v1/repos/:org/:name/pulls/:num/checks                      → [Check]

# Comments
GET    /v1/repos/:org/:name/pulls/:num/comments                    → [Comment]
POST   /v1/repos/:org/:name/pulls/:num/comments                    → Comment
                  { body, path?, line?, side?, in_reply_to? }

# Reviews
POST   /v1/repos/:org/:name/pulls/:num/reviews
                  { verdict: 'comment'|'approve'|'changes', body, comments: [...] }

# Merge
POST   /v1/repos/:org/:name/pulls/:num/merge
                  { strategy: 'merge'|'squash'|'rebase', commit_title?, commit_body? }
```

### 3.4 Developer settings — drives `DevSettingsView` (`src/views/devsettings.jsx`)

**This is the flagship.** The 3-step PAT flow lives or dies here. Endpoints must be atomic.

```
# Personal access tokens
GET    /v1/me/tokens                                               → [Token]      (no secret)
POST   /v1/me/tokens   { name, scopes: [...], expires_in_days }    → TokenCreated (secret ONCE)
POST   /v1/me/tokens/:id/rotate                                    → TokenCreated (secret ONCE)
DELETE /v1/me/tokens/:id                                           → 204

# SSH keys
GET    /v1/me/ssh-keys                                             → [SSHKey]
POST   /v1/me/ssh-keys   { name, key }                             → SSHKey
DELETE /v1/me/ssh-keys/:id                                         → 204

# Webhooks (user-scoped index; per-repo CRUD)
GET    /v1/me/webhooks                                             → [WebhookSummary]   ← yes, flat
GET    /v1/repos/:org/:name/webhooks                               → [Webhook]
POST   /v1/repos/:org/:name/webhooks   { url, events: [...], secret? }
PATCH  /v1/repos/:org/:name/webhooks/:id
DELETE /v1/repos/:org/:name/webhooks/:id
POST   /v1/repos/:org/:name/webhooks/:id/test                      → 204

# OAuth apps (apps authorized by me)
GET    /v1/me/oauth-apps                                           → [OAuthGrant]
DELETE /v1/me/oauth-apps/:id                                       → 204

# Preferences
GET    /v1/me/preferences                                          → Preferences
PATCH  /v1/me/preferences                                          → Preferences
```

**Scope vocabulary** (must match `NewTokenForm` in devsettings.jsx):
```
repo:read     repo:write    repo:admin
actions:read  packages:write
user:read
```

### 3.5 Dashboard — drives `DashboardView` (`src/views/dashboard.jsx`)

```
GET    /v1/me/inbox                                                → [InboxItem]
       # Server decides what's an "inbox item": review_requested, token_expiring,
       # mention, security_alert. Mark as the union of these.
GET    /v1/me/activity                ?limit=                      → [Activity]
GET    /v1/me/pinned                                               → [Repo]
```

### 3.6 Command palette — drives `Palette` (`src/palette.jsx`)

A single endpoint that returns ranked hits across actions, repos, PRs, files, users.
```
GET    /v1/search/palette             ?q=                          → PaletteResult
```
`PaletteResult` shape:
```jsonc
{
  "groups": [
    { "name": "Quick action",  "items": [ { id, label, icon, href, kbd? } ] },
    { "name": "Repositories",  "items": [ { id, label, sub, href } ] },
    { "name": "Pull requests", "items": [ ... ] },
    { "name": "Files",         "items": [ ... ] }   // limit 20, only top repo
  ]
}
```
The Quick-action group is **server-built from a small static list** of canonical actions ("Create token", "Add SSH key", "Add webhook", "New repository", "Copy clone URL"). This is what makes ⌘K → "token" land on PAT creation in one step.

### 3.7 Search (full)

```
GET    /v1/search/code            ?q=&repo=&ref=&lang=             → CodeHits
GET    /v1/search/issues          ?q=&repo=&state=                 → IssueHits
GET    /v1/search/repos           ?q=                              → RepoHits
GET    /v1/search/users           ?q=                              → UserHits
```
Code search shells out to `ripgrep` against the working dir of a bare repo's `objects/` cache, scoped by ref. Cache per `(repo, ref)`.

### 3.8 Realtime (SSE)

```
GET    /v1/stream                  ?topics=inbox,repo:kelp/atlas,pull:kelp/atlas#842
                                                                   → text/event-stream
```
Event types: `inbox.new`, `pull.commented`, `pull.reviewed`, `check.updated`, `push`, `webhook.delivered`, `token.expiring`. Each event includes `topic`, `kind`, and a `payload`.

---

## 4. Data model (PostgreSQL)

This is the canonical schema sketch. Migrations are in `internal/db/migrations/` as numbered `.sql` files.

```sql
-- users & auth
CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  handle        CITEXT UNIQUE NOT NULL,
  email         CITEXT UNIQUE NOT NULL,
  name          TEXT NOT NULL,
  password_hash TEXT,                 -- nullable for OIDC-only users
  is_admin      BOOLEAN NOT NULL DEFAULT FALSE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE oidc_identities (
  user_id    BIGINT REFERENCES users(id) ON DELETE CASCADE,
  provider   TEXT NOT NULL,
  subject    TEXT NOT NULL,
  PRIMARY KEY (provider, subject)
);

CREATE TABLE ssh_keys (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  public_key  TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ
);

CREATE TABLE personal_access_tokens (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  hash         TEXT NOT NULL,                  -- argon2id of token
  prefix       CHAR(8) NOT NULL,               -- 'orc_pat_' suffix peek for lookup
  scopes       TEXT[] NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ                     -- nullable = no expiry
);
CREATE INDEX ON personal_access_tokens (user_id);

-- orgs & repos
CREATE TABLE orgs (
  id         BIGSERIAL PRIMARY KEY,
  handle     CITEXT UNIQUE NOT NULL,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE org_members (
  org_id  BIGINT REFERENCES orgs(id) ON DELETE CASCADE,
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  role    TEXT NOT NULL CHECK (role IN ('owner','admin','member')),
  PRIMARY KEY (org_id, user_id)
);

CREATE TABLE repos (
  id              BIGSERIAL PRIMARY KEY,
  org_id          BIGINT REFERENCES orgs(id) ON DELETE CASCADE,
  owner_user_id   BIGINT REFERENCES users(id) ON DELETE CASCADE,  -- one of org/user is set
  name            CITEXT NOT NULL,
  description     TEXT,
  visibility      TEXT NOT NULL CHECK (visibility IN ('public','private','internal')),
  default_branch  TEXT NOT NULL DEFAULT 'main',
  language        TEXT,
  storage_path    TEXT NOT NULL,                                  -- on-disk bare repo dir
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  pushed_at       TIMESTAMPTZ
);
CREATE UNIQUE INDEX ON repos (COALESCE(org_id, 0), COALESCE(owner_user_id, 0), name);

CREATE TABLE user_repo_pins (
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  repo_id BIGINT REFERENCES repos(id) ON DELETE CASCADE,
  ord     INT NOT NULL,
  PRIMARY KEY (user_id, repo_id)
);

-- pull requests
CREATE TABLE pulls (
  id              BIGSERIAL PRIMARY KEY,
  repo_id         BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  number          INT NOT NULL,                          -- per-repo sequence
  title           TEXT NOT NULL,
  body            TEXT,
  author_id       BIGINT NOT NULL REFERENCES users(id),
  head_branch     TEXT NOT NULL,
  base_branch     TEXT NOT NULL,
  head_sha        CHAR(40) NOT NULL,
  base_sha        CHAR(40) NOT NULL,
  state           TEXT NOT NULL CHECK (state IN ('open','merged','closed')),
  mergeable       TEXT NOT NULL DEFAULT 'unknown',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(repo_id, number)
);

CREATE TABLE pull_reviewers (
  pull_id BIGINT REFERENCES pulls(id) ON DELETE CASCADE,
  user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (pull_id, user_id)
);

CREATE TABLE pull_comments (
  id          BIGSERIAL PRIMARY KEY,
  pull_id     BIGINT NOT NULL REFERENCES pulls(id) ON DELETE CASCADE,
  author_id   BIGINT NOT NULL REFERENCES users(id),
  body        TEXT NOT NULL,
  path        TEXT,                       -- nullable = top-level comment
  line        INT,
  side        TEXT CHECK (side IN ('left','right')),
  in_reply_to BIGINT REFERENCES pull_comments(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pull_reviews (
  id         BIGSERIAL PRIMARY KEY,
  pull_id    BIGINT REFERENCES pulls(id) ON DELETE CASCADE,
  author_id  BIGINT REFERENCES users(id),
  verdict    TEXT NOT NULL CHECK (verdict IN ('comment','approve','changes')),
  body       TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- check runs (results pushed in by external CI via webhook)
CREATE TABLE checks (
  id         BIGSERIAL PRIMARY KEY,
  repo_id    BIGINT REFERENCES repos(id) ON DELETE CASCADE,
  ref        TEXT NOT NULL,
  sha        CHAR(40) NOT NULL,
  name       TEXT NOT NULL,
  status     TEXT NOT NULL CHECK (status IN ('queued','pending','ok','fail','cancelled')),
  external_url TEXT,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);
CREATE INDEX ON checks (repo_id, sha);

-- webhooks
CREATE TABLE webhooks (
  id              BIGSERIAL PRIMARY KEY,
  repo_id         BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  url             TEXT NOT NULL,
  secret          TEXT,
  events          TEXT[] NOT NULL,
  active          BOOLEAN NOT NULL DEFAULT TRUE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_delivery_at TIMESTAMPTZ,
  last_delivery_status TEXT
);

-- activity feed (denormalised for fast read)
CREATE TABLE activity (
  id         BIGSERIAL PRIMARY KEY,
  actor_id   BIGINT REFERENCES users(id),
  kind       TEXT NOT NULL,            -- 'pr_review_requested', 'commit', 'deploy', ...
  target     TEXT NOT NULL,            -- 'kelp/atlas#842' style
  title      TEXT NOT NULL,
  payload    JSONB,
  visible_to BIGINT[] NOT NULL,        -- user ids
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON activity USING gin (visible_to);
```

---

## 5. Auth & the PAT flow (the part you must get right)

The frontend's "create token, copy it, done" experience is non-negotiable. Backend rules:

1. **`POST /v1/me/tokens`** must return the plaintext token **exactly once**, in the response body.
2. Token format: `orc_pat_<22 base62 chars>`. First 8 chars (`orc_pat_`) are the fixed prefix used for lookup short-circuiting.
3. Store **only** `argon2id(token)`. Never log the secret. Never write it to anywhere persistent.
4. On every authenticated request that uses a PAT:
   - Parse `Authorization: Bearer orc_pat_…`
   - SELECT all rows for that user prefix (cheap; almost always 1)
   - Argon2id-verify
   - Update `last_used_at` async (do not block the request)
   - Enforce `scopes` against the route's required scope
5. Expiring tokens: a daily job emits an `inbox.token_expiring` activity item 30 / 14 / 3 days before expiry. The Inbox card in `DashboardView` consumes this.

SSH-key auth piggybacks on the same `personal_access_tokens` table's siblings: `ssh_keys.public_key` is matched at SSH-server connect time; we map fingerprint → `user_id`, then enforce repo ACLs per-push.

---

## 6. Git operations

- **Repo storage**: bare repos under `${STORAGE_DIR}/repos/<repo_id>.git`. Repo IDs are integers; never expose paths to the user.
- **Reads**: `go-git` for tree, blob, log, diff. Cache `(repo, ref, op)` results for 30s in-memory.
- **Writes**: shell out to `git` (`git receive-pack`, `git upload-pack`, `git push`, `git merge`). Reason: go-git's write path is incomplete around pack negotiation.
- **Hooks**: install a `post-receive` shell hook in every repo that POSTs back to `localhost:PORT/internal/hooks/push` with `repo_id`, `ref`, `old`, `new`. This is what fans out webhooks, updates `pushed_at`, refreshes PR `head_sha`, recomputes mergeability.
- **PR diff**: compute on the fly from `base_sha` vs `head_sha` with `git diff --unified=3`. Cache per `(base, head)`. The shape returned must match `PR_DIFF_FILES` in `src/data.jsx`.

---

## 7. Realtime (SSE)

`GET /v1/stream` keeps the connection open, subscribes to a per-process pub-sub (`hashicorp/go-memberlist`-free; just an in-process channel registry) and writes `event: …\ndata: …\n\n`. On every mutation (PR comment, push, check update) the handler publishes to a topic. Subscriptions are gated by ACL.

Heartbeat every 25s as a `: ping` comment to defeat proxies. Reconnect with `Last-Event-Id` resumes from a small ring buffer (1k events) to dodge UI flicker on transient drops.

---

## 8. Search

- **`/v1/search/palette`** — small, fast. SELECT TOP 20 across:
  - canonical actions (static list, filtered by ACL)
  - `repos` ILIKE `%q%` on `name` and `description`
  - `pulls` FTS on `title` (where user has read access)
  - filenames from a small per-repo `tree_index` table (path, repo_id, ref='HEAD') refreshed by the push hook
  Cap each group at 6.
- **`/v1/search/code`** — shells out to `rg --json --max-count=50 --type-add 'rust:*.rs' …` against the worktree-cache of the repo. Stream the JSONL back, transform into hits.
- **Postgres FTS** is enough for issues/PRs for the first ~10M rows. Consider tantivy later if needed; do not premature-optimise.

---

## 9. Webhooks (outbound)

A single worker goroutine reads from a `webhook_deliveries` queue table. Each delivery:
1. POST JSON with `X-Orchis-Event`, `X-Orchis-Delivery` (uuid), `X-Orchis-Signature` (`hmac-sha256(secret, body)`).
2. On non-2xx: exponential backoff (1s, 4s, 16s, 1m, 5m, 30m). Give up after 6 attempts.
3. Log every delivery in `webhook_deliveries` for the UI's "last delivery" indicator.

---

## 10. Config

A single YAML, env vars override. Example `orchis.yaml`:
```yaml
server:
  http_addr: ":8080"
  ssh_addr: ":2222"
  external_url: "https://orchis.example.com"
database:
  url: "postgres://orchis:orchis@localhost:5432/orchis?sslmode=disable"
storage:
  repos_dir: "/var/lib/orchis/repos"
  blobs:
    kind: "fs"          # or "s3"
    fs_root: "/var/lib/orchis/blobs"
auth:
  session_key: "${ORCHIS_SESSION_KEY}"        # 32 random bytes, base64
  pat_argon2:
    memory_kib: 65536
    iterations: 3
    parallelism: 2
  oidc:
    - id: github
      issuer: "https://github.com"
      client_id: "${GITHUB_CLIENT_ID}"
      client_secret: "${GITHUB_CLIENT_SECRET}"
```

---

## 11. Implementation order

Follow this sequence. Each step ends with a runnable, demoable milestone.

1. **Bootstrap** — `go.mod`, chi router, `/healthz`, static-serve `web/`. App loads in the browser, hits no real endpoints.
2. **DB + users + sessions** — login, `/v1/me`, cookie auth.
3. **Repos (read-only)** — list, get, tree, blob, raw. Wire `RepoView` to real data.
4. **PATs end-to-end** — token CRUD + middleware. **Verify the 3-step flow works.**
5. **SSH server** — git push/pull over SSH using stored keys.
6. **HTTP git** — `/v1/repos/:org/:name/git/{upload,receive}-pack` with Basic auth (handle + PAT).
7. **Pulls** — list, detail, files (diff), comments, reviews, merge.
8. **Webhooks** — CRUD + delivery worker.
9. **SSE stream** — wire `/v1/stream`, push events from mutations.
10. **Command palette** — `/v1/search/palette` with the quick-action group.
11. **Activity & inbox** — denormalised feed, expiring-token job.
12. **OIDC** — at least GitHub + generic.
13. **Code search** — ripgrep wrapper.
14. **Docker compose** — single-command self-host.

Stop and demo after **step 4**. The PAT flow is the proof the design works.

---

## 12. Testing

- Unit tests live next to source: `foo.go` + `foo_test.go`. Use stdlib `testing` only.
- Integration tests under `internal/api/*_integration_test.go` boot a real Postgres via `testcontainers-go` and exercise the HTTP layer end-to-end.
- Golden-file diff tests for the PR diff shape — keep them locked to `src/data.jsx`'s `PR_DIFF_FILES`. If you change the shape, you change the frontend; don't.
- `make smoke` runs a tiny scripted demo: create user, create repo, push via SSH, open PR via API, comment, merge.

---

## 13. Don't surprise the frontend

The JSX is the spec. Before changing any API shape:
1. Search the frontend (`grep -r 'fieldName' src/`) for usage.
2. If it's used, the shape stays. Add new fields; never repurpose.
3. Frontend keys are camelCase. Backend JSON tags must match.

When in doubt, open `src/data.jsx` and read the mock — that's the contract.

---

## 14. License & posture

MIT. This is a self-hostable platform; no telemetry without an opt-in flag. Default config emits zero outbound calls except for OIDC if configured.

---

*If you're Claude Code reading this — start at §11 step 1, get the static site serving, and proceed in order. Don't skip ahead to fancy features; the value of this project is in the 3-step rule, and the 3-step rule lives in PATs (§5) and the command palette (§3.6).*
