CREATE TABLE IF NOT EXISTS wedding_banners (
    wedding_id text PRIMARY KEY REFERENCES weddings(id) ON DELETE CASCADE,
    version text NOT NULL,
    image bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
