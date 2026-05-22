-- 0007_issues: lightweight issue tracker (issues + comments).

CREATE TABLE issues (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  number      INTEGER NOT NULL,                 -- per-repo sequence
  title       TEXT NOT NULL,
  body        TEXT NOT NULL DEFAULT '',
  author_id   INTEGER NOT NULL REFERENCES users(id),
  assignee_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  state       TEXT NOT NULL DEFAULT 'open',     -- open | closed
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
  closed_at   TEXT,
  UNIQUE(repo_id, number)
);
CREATE INDEX idx_issues_repo ON issues(repo_id, state);

CREATE TABLE issue_comments (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  issue_id   INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
  author_id  INTEGER NOT NULL REFERENCES users(id),
  body       TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_issue_comments ON issue_comments(issue_id);
