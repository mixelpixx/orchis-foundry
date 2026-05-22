-- 0008_stars: per-user repo stars (pins already exist in 0002).

CREATE TABLE user_repo_stars (
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  repo_id    INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (user_id, repo_id)
);
CREATE INDEX idx_stars_repo ON user_repo_stars(repo_id);
