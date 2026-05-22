-- 0005_activity: denormalized activity feed for the dashboard.

CREATE TABLE activity (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
  kind       TEXT NOT NULL,             -- repo_created, pr_opened, pr_merged, pr_reviewed, pr_commented, scan_flagged
  repo_id    INTEGER REFERENCES repos(id) ON DELETE CASCADE,
  target     TEXT NOT NULL DEFAULT '',  -- e.g. "owner/name#7" or "owner/name"
  title      TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_activity_repo ON activity(repo_id);
CREATE INDEX idx_activity_id ON activity(id DESC);
