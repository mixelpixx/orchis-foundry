# Orchis Foundry

A modern, **self-hosted code platform** — calm, minimal UI, Apple's 3-step rule
enforced. Git hosting, pull requests, issues, releases, per-repo collaborators,
a per-user BYO-model supply-chain scanner, realtime updates, and LLM-friendly
context endpoints. **One static Go binary, one config file, one SQLite database.**

Live instance: **https://foundry.orchis.ai**

---

## What it does

**Auth & access**
- **Sign in three ways**, each toggled by an admin: local **email + password**
  (Argon2id), any standards-compliant **OIDC** provider (Microsoft Entra ID,
  Google, Okta, Keycloak, …), and **GitHub**. The sign-in screen shows only the
  methods that are enabled.
- **Admin panel** (instance admins) to choose which sign-in methods are on and to
  manage users — create accounts, grant/remove admin, **disable (deprovision)**,
  reset passwords. Disabling a user immediately revokes their sessions.
- DB-backed opaque sessions (HttpOnly, Secure, SameSite cookies; IP/UA-bound).
- Personal access tokens (`orc_pat_…`, Argon2id-hashed, scoped, shown once).
- SSH keys for git over SSH.
- **Per-repo RBAC** — collaborators with `read` / `write` / `admin` roles; the
  owner is an implicit admin. Enforced everywhere: HTTP + SSH push, merge,
  branches, releases, settings, webhooks, collaborator management.

**Repositories**
- Create / browse (file tree, blob, raw, README), branches (create/delete/set
  default), tags, commit history + single-commit diff, branch **compare**.
- **Releases** — publish from a tag with notes; download source `.tar.gz`.
- Clone/push over **HTTPS** and **SSH** (`:2222`).

**Pull requests**
- **Open a PR from the browser** (compare base/head → title/body → reviewers).
- Diff with **inline line comments**, **batched reviews** (stage comments +
  verdict), request reviewers, merge (merge / squash / rebase).
- Check runs (incl. the supply-chain scan) shown per PR.

**Issues** — open / comment / assign / close.

**Webhooks** — per-repo, HMAC-SHA256 signed delivery with retry/backoff.

**Supply-chain scanner + AI assist** — each user configures one BYO model
(Anthropic or any OpenAI-compatible endpoint). It powers the `orchis-scan` PR
check *and* in-PR AI (summarize / explain / draft description).

**Realtime** — `/v1/stream` (SSE) drives live PR updates, check flips, and the
notifications bell.

**Search** — ⌘K command palette (repos / PRs / files / quick actions) + full
**code search** (`git grep`) + repo/user search.

**LLM-friendly** — `/llms.txt`, `/v1/openapi.json`, and `…/pack` endpoints that
flatten a repo or PR into one markdown document for ingestion.

---

## Who it's for & scope

Orchis Foundry is a **self-hosted Git + code-review platform for solo developers
and small teams** who want their code in-house, with an AI review gate and
supply-chain scanning built in. It is **not** an enterprise GitHub replacement —
it is honest about that, and the gaps below are the documented roadmap
(see [`ROADMAP.md`](ROADMAP.md)), not hidden surprises.

| Included today | Not yet (roadmap) |
|---|---|
| Git over HTTPS + SSH, branches, tags, releases | Organizations / teams (per-repo collaborators only) |
| PRs: inline comments, batched reviews, merge/squash/rebase | Branch protection / required-review merge gating |
| Issues, webhooks (HMAC, retries) | Native CI/CD ("Actions"), package registry, Git LFS |
| Local email+password, OIDC (Entra/Google/Okta/…), GitHub | SAML; SCIM auto-provisioning |
| Admin UI: enable sign-in methods, create/disable/deprovision users | Audit log; self-service password reset (needs email) |
| Per-repo RBAC (read/write/admin); Argon2id PATs; SSH keys | Email notifications, 2FA/MFA |
| AI change-proposal review gate + supply-chain scanner (BYO model) | Prometheus metrics |
| Code search (`git grep`), ⌘K palette, SSE realtime | Postgres backend (SQLite only today) |
| Daily SQLite backup timer, single-binary + Docker deploy | Horizontal scale / multi-tenant SaaS |

> Sized for teams up to ~25–50 active users on a single node. For larger or
> regulated deployments, see the roadmap items (SSO/SAML, audit log, Postgres).

---

## Authentication (how it works)

Three sign-in methods, all optional and toggled by an instance admin in
**Admin → Authentication**:

- **Email + password** — local accounts. Passwords are hashed with Argon2id
  (never stored in plaintext). "Open sign-up" lets anyone register; leave it off
  and have an admin create accounts instead.
- **OIDC** — any standards-compliant provider. Configure it in `orchis.yaml`
  under `oidc:` with the provider's **issuer URL**, **client ID**, and **client
  secret**; the server discovers the rest and verifies the ID token's signature.
  Examples: Microsoft **Entra ID** (`issuer: https://login.microsoftonline.com/<tenant>/v2.0`),
  **Google** (`https://accounts.google.com`), Okta, Keycloak.
- **GitHub** — built-in OAuth2 (GitHub isn't a compliant OIDC issuer, so it has
  its own path).

```yaml
# orchis.yaml — add the providers you want; they appear on the sign-in screen
oidc:
  - id: entra
    issuer: https://login.microsoftonline.com/<tenant-id>/v2.0
    client_id: "<app-client-id>"
    client_secret: "${ENTRA_CLIENT_SECRET}"
  - id: google
    issuer: https://accounts.google.com
    client_id: "<...>.apps.googleusercontent.com"
    client_secret: "${GOOGLE_CLIENT_SECRET}"
```

Notes:
- The **first user** to sign in (any method) becomes the instance admin. Bootstrap
  a fresh instance without a browser via `orchis-foundry --mint-token <name>`.
- A redirect/callback is served at `/v1/auth/oidc/callback`; register that URL
  with each provider. `external_url` in config must be the public base URL.
- Sessions are opaque, server-side, and revoked immediately when a user is
  disabled. Client secrets live only in config — never in the database or UI.

---

## Stack

| Layer | Choice |
|---|---|
| Language | Go 1.25, single static binary (`CGO_ENABLED=0`) |
| HTTP | `go-chi/chi` |
| DB | SQLite (`modernc.org/sqlite`, pure-Go) + in-process SQL migrations |
| Git | `go-git` for reads; shell out to system `git` for writes / pack / grep / archive |
| SSH | `gliderlabs/ssh`, embedded, port 2222 |
| Auth | Argon2id PATs, DB sessions, GitHub OAuth |
| Realtime | Server-Sent Events (in-process pub/sub) |
| Frontend | React 18 (precompiled), served from the binary |

### Frontend build
The UI is React authored as `src/*.jsx` and **precompiled** into a single
bundle (no in-browser Babel, no CDN). Source of truth is `web/src/*.jsx`;
rebuild with:

```bash
cd build && npm install && npm run build   # → web/app.bundle.js + web/vendor/
```

`web/` (index.html + app.bundle.js + vendor) is embedded into the binary via
`go:embed`. CSP is locked to `script-src 'self'`.

---

## Quick start (Docker)

```bash
docker compose up --build      # then open http://localhost:8080
```

Edit `ORCHIS_SESSION_KEY` in `docker-compose.yml` first (`openssl rand -base64 32`).
Web-UI login needs an OIDC provider — see the commented `oidc:` config mount in
`docker-compose.yml` and `docs/deploy.md`. To exercise the API/git without OIDC:

```bash
docker compose exec orchis orchis-foundry --mint-token admin   # prints a PAT
```

…or just click around the hosted demo at **https://foundry.orchis.ai**.

## Build & run (local)

```bash
# Build the binary (frontend bundle is committed under web/)
CGO_ENABLED=0 go build -o orchis-foundry ./cmd/orchis

# Run with env overrides (or a config.yaml — see internal/config)
ORCHIS_HTTP_ADDR=127.0.0.1:8090 \
ORCHIS_DB_PATH=/tmp/foundry.db \
ORCHIS_REPOS_DIR=/tmp/repos \
ORCHIS_SESSION_KEY=$(head -c32 /dev/urandom | base64) \
./orchis-foundry

# Mint a token for local API/git testing without OIDC:
./orchis-foundry --mint-token alice    # prints user=alice token=orc_pat_…
```

Config keys (`config.yaml`, env overrides win): `ORCHIS_HTTP_ADDR`,
`ORCHIS_SSH_ADDR`, `ORCHIS_EXTERNAL_URL`, `ORCHIS_DB_PATH`, `ORCHIS_REPOS_DIR`,
`ORCHIS_SSH_HOST_KEY`, `ORCHIS_SESSION_KEY`, plus OIDC + scanner settings.

## Tests

```bash
go test ./...        # unit + golden (PR diff shape) + ACL + git-layer (grep/branch/commit) tests
```

## API

Authenticate with `Authorization: Bearer orc_pat_<…>`. The full surface is
self-describing:

- `GET /v1/openapi.json` — OpenAPI 3.1 for every `/v1` route
- `GET /llms.txt` — agent-oriented index
- `GET /v1/repos/{org}/{name}/pack` — whole repo as one markdown doc

## Deploy & ops

See **`docs/deploy.md`** for the production runbook (systemd unit, nginx, TLS,
daily SQLite backup timer, and the build → cross-compile → swap → verify loop).

## License

MIT. No telemetry; zero outbound calls except OIDC and your configured model.
