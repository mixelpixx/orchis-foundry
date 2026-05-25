-- 0015_ai_chats: persistent per-repo AI chat (the slide-out sidebar). Optional
-- feature; the whole panel is gated by the user's `aiChat` preference.
CREATE TABLE ai_chats (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  repo_id    INTEGER NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
  title      TEXT NOT NULL DEFAULT 'New chat',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_ai_chats_user_repo ON ai_chats(user_id, repo_id, id);

CREATE TABLE ai_chat_messages (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  chat_id    INTEGER NOT NULL REFERENCES ai_chats(id) ON DELETE CASCADE,
  role       TEXT NOT NULL CHECK (role IN ('user','assistant')),
  content    TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_ai_chat_messages_chat ON ai_chat_messages(chat_id, id);
