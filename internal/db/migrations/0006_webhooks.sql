-- 0006_webhooks: outbound webhooks + delivery queue.

CREATE TABLE webhooks (
  id                   INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id              INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  url                  TEXT NOT NULL,
  secret               TEXT NOT NULL DEFAULT '',  -- HMAC-SHA256 signing key (optional)
  events               TEXT NOT NULL DEFAULT '',  -- comma-separated: push,pull_request,...
  active               INTEGER NOT NULL DEFAULT 1,
  created_at           TEXT NOT NULL DEFAULT (datetime('now')),
  last_delivery_at     TEXT,
  last_delivery_status TEXT                        -- 'ok' | 'fail'
);
CREATE INDEX idx_webhooks_repo ON webhooks(repo_id);

CREATE TABLE webhook_deliveries (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  webhook_id      INTEGER NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
  uuid            TEXT NOT NULL,
  event           TEXT NOT NULL,
  payload         TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'queued',  -- queued | delivered | failed
  attempts        INTEGER NOT NULL DEFAULT 0,
  response_code   INTEGER,
  error           TEXT,
  next_attempt_at TEXT NOT NULL DEFAULT (datetime('now')),
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  delivered_at    TEXT
);
CREATE INDEX idx_wh_deliveries_pending ON webhook_deliveries(status, next_attempt_at);
