-- Platform account moderation state. Existing accounts remain active.
ALTER TABLE users ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';
ALTER TABLE users ADD CONSTRAINT users_status_check CHECK (status IN ('active', 'suspended'));
CREATE INDEX IF NOT EXISTS users_status_idx ON users (status);
