CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token_hash text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz
);
CREATE INDEX IF NOT EXISTS password_reset_tokens_user_idx ON password_reset_tokens (user_id);
CREATE INDEX IF NOT EXISTS password_reset_tokens_expiry_idx ON password_reset_tokens (expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS password_reset_tokens_one_active_user_idx
    ON password_reset_tokens (user_id) WHERE used_at IS NULL;
