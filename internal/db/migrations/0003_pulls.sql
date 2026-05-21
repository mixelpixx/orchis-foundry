-- 0003_pulls: pull requests, comments, reviews, reviewers, checks.

CREATE TABLE pulls (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id      INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  number       INTEGER NOT NULL,
  title        TEXT NOT NULL,
  body         TEXT NOT NULL DEFAULT '',
  author_id    INTEGER NOT NULL REFERENCES users(id),
  head_branch  TEXT NOT NULL,
  base_branch  TEXT NOT NULL,
  head_sha     TEXT NOT NULL DEFAULT '',
  base_sha     TEXT NOT NULL DEFAULT '',
  state        TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','merged','closed')),
  additions    INTEGER NOT NULL DEFAULT 0,
  deletions    INTEGER NOT NULL DEFAULT 0,
  files_count  INTEGER NOT NULL DEFAULT 0,
  commits_count INTEGER NOT NULL DEFAULT 0,
  created_at   TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at   TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(repo_id, number)
);
CREATE INDEX idx_pulls_repo ON pulls(repo_id);
CREATE INDEX idx_pulls_author ON pulls(author_id);

CREATE TABLE pull_reviewers (
  pull_id INTEGER NOT NULL REFERENCES pulls(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (pull_id, user_id)
);

CREATE TABLE pull_comments (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  pull_id     INTEGER NOT NULL REFERENCES pulls(id) ON DELETE CASCADE,
  author_id   INTEGER NOT NULL REFERENCES users(id),
  body        TEXT NOT NULL,
  path        TEXT,
  line        INTEGER,
  side        TEXT CHECK (side IN ('left','right')),
  in_reply_to INTEGER REFERENCES pull_comments(id),
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_pull_comments_pull ON pull_comments(pull_id);

CREATE TABLE pull_reviews (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  pull_id    INTEGER NOT NULL REFERENCES pulls(id) ON DELETE CASCADE,
  author_id  INTEGER NOT NULL REFERENCES users(id),
  verdict    TEXT NOT NULL CHECK (verdict IN ('comment','approve','changes')),
  body       TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE checks (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  repo_id     INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  sha         TEXT NOT NULL,
  name        TEXT NOT NULL,
  status      TEXT NOT NULL CHECK (status IN ('queued','pending','ok','fail','cancelled')),
  detail      TEXT NOT NULL DEFAULT '',
  external_url TEXT,
  started_at  TEXT,
  finished_at TEXT,
  UNIQUE(repo_id, sha, name)
);
CREATE INDEX idx_checks_repo_sha ON checks(repo_id, sha);
