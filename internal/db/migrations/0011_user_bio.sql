-- 0011_user_bio: a short profile bio users can edit.
ALTER TABLE users ADD COLUMN bio TEXT NOT NULL DEFAULT '';
