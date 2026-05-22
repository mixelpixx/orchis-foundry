# Deploy & operations runbook

Orchis Foundry ships as one static binary that embeds the frontend. Production
runs behind nginx (TLS) under systemd, with a daily SQLite backup timer.

## Layout on the host

```
/usr/local/bin/orchis-foundry            the binary
/etc/orchis-foundry/config.yaml          config (DB path, OIDC, session key…)
/var/lib/orchis-foundry/foundry.db       SQLite database (WAL)
/var/lib/orchis-foundry/repos/<id>.git   bare repos
/var/backups/orchis-foundry/             daily .db.gz snapshots (14 kept)
```

Runs as the unprivileged `foundry` user. nginx terminates TLS and proxies to
`127.0.0.1:8090`; the embedded SSH git server listens on `:2222`.

## systemd

Tracked units in `scripts/`:

- `orchis-foundry.service` — the app. Hardened: `ProtectSystem=strict`,
  `ReadWritePaths=/var/lib/orchis-foundry …`, `ProtectHome=true`,
  `NoNewPrivileges=true`, `PrivateTmp=true` (the PR-merge path needs a writable
  `/tmp` for a throwaway worktree).
- `orchis-foundry-backup.{service,timer}` + `orchis-foundry-backup.sh` — daily
  `sqlite3 .backup` at 03:30, gzipped, 14-day retention.

Install / refresh:

```bash
cp scripts/orchis-foundry.service /etc/systemd/system/
cp scripts/orchis-foundry-backup.{service,timer} /etc/systemd/system/
cp scripts/orchis-foundry-backup.sh /usr/local/bin/ && chmod 755 /usr/local/bin/orchis-foundry-backup.sh
systemctl daemon-reload
systemctl enable --now orchis-foundry
systemctl enable --now orchis-foundry-backup.timer
```

## nginx

A standard TLS reverse proxy to `127.0.0.1:8090`. Two notes that matter:

- **SSE**: the `/v1/stream` endpoint sets `X-Accel-Buffering: no`; nginx honors
  it, so events are not buffered. Keep `proxy_read_timeout` ≥ 60s (the stream
  heartbeats every 25s).
- **Git push** can send large bodies — set `client_max_body_size` generously
  (e.g. `512m`) on the git endpoints.

## Deploy loop (cross-compile from dev → swap on host)

The frontend bundle is committed under `web/`, so a deploy is just the binary.
If you changed any `src/*.jsx`, **rebuild the bundle first** (`cd build && npm
run build`) and commit it.

```bash
# 1. Cross-compile for the VPS (linux/amd64), version-stamped:
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-X main.version=X.Y.Z -s -w" -o orchis-foundry.linux ./cmd/orchis

# 2. Copy up, back up current binary + DB, swap, restart:
scp orchis-foundry.linux root@HOST:/usr/local/bin/orchis-foundry.new
ssh root@HOST '
  cp -a /usr/local/bin/orchis-foundry /usr/local/bin/orchis-foundry.bak-$(date +%s)
  cp -a /var/lib/orchis-foundry/foundry.db /var/lib/orchis-foundry/foundry.db.bak-$(date +%s)
  chmod 755 /usr/local/bin/orchis-foundry.new
  mv /usr/local/bin/orchis-foundry.new /usr/local/bin/orchis-foundry
  systemctl restart orchis-foundry'

# 3. Verify:
ssh root@HOST '/usr/local/bin/orchis-foundry --version; \
  curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8090/healthz'
```

DB migrations run automatically in-process on startup (embedded
`internal/db/migrations/*.sql`); the pre-swap DB backup is the rollback safety
net. To roll back: `mv` the `.bak` binary back and `systemctl restart`.

## Health & smoke

- `GET /healthz` → 200.
- `--version` prints the build stamp.
- Public sanity: the four `*.orchis.ai` sites should all return 200.
- Token for live testing: `orchis-foundry --config … --mint-token <handle>`
  (clean up the throwaway user/repo afterward).

## Backups & restore

Snapshots live in `/var/backups/orchis-foundry/foundry-YYYYmmdd-HHMMSS.db.gz`.
To restore: stop the service, `gunzip` a snapshot over `foundry.db` (remove
`-wal`/`-shm` first), restart.

## Secrets

Live only in `/etc/orchis-foundry/config.yaml` on the host (session key, OIDC
client secret, etc.) — never committed. The repo's `credentials/` equivalent is
intentionally kept out of version control.
