ALTER TABLE invitations ADD COLUMN IF NOT EXISTS short_code_hash text;
CREATE UNIQUE INDEX IF NOT EXISTS invitations_short_code_hash_idx ON invitations (short_code_hash) WHERE short_code_hash IS NOT NULL;
