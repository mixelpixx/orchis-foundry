-- 0014_change_proposals: the human-in-the-loop gate for AI/code changes.
-- An LLM (or any client) proposes a change; nothing touches a branch or opens a
-- PR until a human with write access accepts it. A proposal carries EITHER a
-- unified-diff `patch` OR a `changes` JSON array of {path,content,delete}.
CREATE TABLE change_proposals (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id       INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  author_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title         TEXT NOT NULL,
  summary       TEXT NOT NULL DEFAULT '',
  base_sha      TEXT NOT NULL DEFAULT '',
  target_branch TEXT NOT NULL DEFAULT '',
  patch         TEXT NOT NULL DEFAULT '',   -- unified diff (optional)
  changes       TEXT NOT NULL DEFAULT '',   -- JSON [{path,content,delete}] (optional)
  status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','rejected')),
  applied_sha   TEXT NOT NULL DEFAULT '',
  pull_number   INTEGER,
  created_at    TEXT NOT NULL DEFAULT (datetime('now')),
  resolved_at   TEXT
);
CREATE INDEX idx_change_proposals_repo ON change_proposals(repo_id, status, id);
