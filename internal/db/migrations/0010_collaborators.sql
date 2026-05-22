-- 0010_collaborators: per-repo collaborators with read/write/admin roles.
-- The repo owner (repos.owner_user_id) is an implicit admin and is NOT stored
-- here. Roles are a strict ladder: read < write < admin.

CREATE TABLE repo_collaborators (
  repo_id    INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       TEXT NOT NULL CHECK (role IN ('read','write','admin')),
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (repo_id, user_id)
);
CREATE INDEX idx_collab_user ON repo_collaborators(user_id);
