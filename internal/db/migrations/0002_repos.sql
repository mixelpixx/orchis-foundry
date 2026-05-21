-- 0002_repos: orgs (forward-compat), repos, pins.
-- v1 simplification: repos are user-owned (owner_user_id set, org_id null).
-- Org support is schema-ready but not yet wired in handlers.

CREATE TABLE orgs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  handle     TEXT NOT NULL UNIQUE COLLATE NOCASE,
  name       TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE org_members (
  org_id  INTEGER NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role    TEXT NOT NULL CHECK (role IN ('owner','admin','member')),
  PRIMARY KEY (org_id, user_id)
);

CREATE TABLE repos (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  org_id         INTEGER REFERENCES orgs(id) ON DELETE CASCADE,
  owner_user_id  INTEGER REFERENCES users(id) ON DELETE CASCADE,
  name           TEXT NOT NULL COLLATE NOCASE,
  description    TEXT NOT NULL DEFAULT '',
  visibility     TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('public','private','internal')),
  default_branch TEXT NOT NULL DEFAULT 'main',
  language       TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL DEFAULT (datetime('now')),
  pushed_at      TEXT
);
CREATE UNIQUE INDEX idx_repos_owner_name ON repos (IFNULL(org_id,0), IFNULL(owner_user_id,0), name);

CREATE TABLE user_repo_pins (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  repo_id INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  ord     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, repo_id)
);
