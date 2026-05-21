-- 0004_scanner: per-user scanner config + scan job queue + findings.

-- Per-user scanner configuration (BYO endpoint).
CREATE TABLE scanner_settings (
  user_id    INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  enabled    INTEGER NOT NULL DEFAULT 0,
  provider   TEXT NOT NULL DEFAULT 'anthropic' CHECK (provider IN ('anthropic','openai_compatible')),
  base_url   TEXT NOT NULL DEFAULT '',   -- for openai_compatible (e.g. http://host:11434/v1)
  model      TEXT NOT NULL DEFAULT '',   -- e.g. claude-sonnet-4-5 | qwen2.5-coder | gpt-4o-mini
  api_key    TEXT NOT NULL DEFAULT '',   -- stored at rest; never returned to the client
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Scan job queue. One row per (pull, head_sha) scan request.
CREATE TABLE scan_jobs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  pull_id     INTEGER NOT NULL REFERENCES pulls(id) ON DELETE CASCADE,
  repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  sha         TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','done','error')),
  error       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  started_at  TEXT,
  finished_at TEXT
);
CREATE INDEX idx_scan_jobs_status ON scan_jobs(status, id);

-- Structured findings from a completed scan.
CREATE TABLE scan_findings (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id     INTEGER NOT NULL REFERENCES scan_jobs(id) ON DELETE CASCADE,
  severity   TEXT NOT NULL,
  category   TEXT NOT NULL,
  file       TEXT NOT NULL DEFAULT '',
  line_start INTEGER,
  line_end   INTEGER,
  summary    TEXT NOT NULL,
  suggestion TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_scan_findings_job ON scan_findings(job_id);
