-- 0012_user_prefs: per-user UI preferences (theme/accent/density/font…) as JSON.
ALTER TABLE users ADD COLUMN prefs TEXT NOT NULL DEFAULT '{}';
