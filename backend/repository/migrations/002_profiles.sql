-- Self-service profiles for a wedding's admin and committee members.
--
-- profile_key is 'admin' for the wedding admin capability holder, or the committee
-- member id for a committee member, so every actor owns exactly one profile per
-- wedding. avatar stores a small, browser-resized image data URL; the API rejects
-- anything oversized so a profile row stays compact.
CREATE TABLE IF NOT EXISTS profiles (
    wedding_id   text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    profile_key  text NOT NULL,
    role         text NOT NULL DEFAULT '',
    display_name text NOT NULL DEFAULT '',
    avatar       text NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL,
    PRIMARY KEY (wedding_id, profile_key)
);
