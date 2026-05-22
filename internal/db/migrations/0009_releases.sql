-- 0009_releases: release metadata layered on top of git tags.

CREATE TABLE releases (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  tag         TEXT NOT NULL,
  name        TEXT NOT NULL DEFAULT '',
  body        TEXT NOT NULL DEFAULT '',
  author_id   INTEGER REFERENCES users(id) ON DELETE SET NULL,
  prerelease  INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(repo_id, tag)
);
CREATE INDEX idx_releases_repo ON releases(repo_id);
