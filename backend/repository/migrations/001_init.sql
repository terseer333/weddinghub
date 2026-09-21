-- WeddingHub initial schema.
--
-- The wedding is the consistency boundary: every content table carries a
-- wedding_id foreign key with ON DELETE CASCADE so deleting a wedding removes
-- its whole aggregate. Invitation token hashes and admin capability hashes are
-- compared by equality, so they live in uniquely indexed text columns and are
-- never stored in plaintext.
--
-- Nullable timestamps (date, expires_at, responded_at, published_at, ends_at)
-- stay NULL. Permissive text columns default to '' so the Go models can scan
-- them into plain strings without nullable wrappers.

CREATE TABLE IF NOT EXISTS users (
    id            text PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    display_name  text NOT NULL DEFAULT '',
    role          text NOT NULL DEFAULT 'owner',
    password_hash text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash text PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions (user_id);
CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE IF NOT EXISTS weddings (
    id               text PRIMARY KEY,
    slug             text NOT NULL UNIQUE,
    title            text NOT NULL DEFAULT '',
    partner_one      text NOT NULL DEFAULT '',
    partner_two      text NOT NULL DEFAULT '',
    date             timestamptz,
    status           text NOT NULL,
    venue            text NOT NULL DEFAULT '',
    address          text NOT NULL DEFAULT '',
    city             text NOT NULL DEFAULT '',
    state            text NOT NULL DEFAULT '',
    country          text NOT NULL DEFAULT '',
    message          text NOT NULL DEFAULT '',
    verse            text NOT NULL DEFAULT '',
    dress_code       text NOT NULL DEFAULT '',
    hero_image       text NOT NULL DEFAULT '',
    template_id      text NOT NULL DEFAULT '',
    card_config      jsonb,
    admin_token_hash text NOT NULL,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS weddings_admin_token_hash_idx ON weddings (admin_token_hash);

CREATE TABLE IF NOT EXISTS wedding_admins (
    wedding_id text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    user_id    text NOT NULL,
    role       text NOT NULL,
    PRIMARY KEY (wedding_id, user_id)
);

CREATE TABLE IF NOT EXISTS committee_roles (
    id          text PRIMARY KEY,
    wedding_id  text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_custom   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL,
    position    integer NOT NULL DEFAULT 0,
    UNIQUE (wedding_id, name)
);
CREATE INDEX IF NOT EXISTS committee_roles_wedding_idx ON committee_roles (wedding_id, position, id);

CREATE TABLE IF NOT EXISTS guests (
    id            text PRIMARY KEY,
    wedding_id    text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    invitation_id text NOT NULL DEFAULT '',
    name          text NOT NULL DEFAULT '',
    email         text NOT NULL DEFAULT '',
    phone         text NOT NULL DEFAULT '',
    position      integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS guests_wedding_idx ON guests (wedding_id, position, id);
CREATE INDEX IF NOT EXISTS guests_invitation_idx ON guests (wedding_id, invitation_id);

CREATE TABLE IF NOT EXISTS committee_members (
    id            text PRIMARY KEY,
    wedding_id    text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    invitation_id text NOT NULL DEFAULT '',
    name          text NOT NULL DEFAULT '',
    email         text NOT NULL DEFAULT '',
    phone         text NOT NULL DEFAULT '',
    title         text NOT NULL DEFAULT '',
    role_id       text NOT NULL DEFAULT '',
    joined_at     timestamptz NOT NULL,
    position      integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS committee_members_wedding_idx ON committee_members (wedding_id, position, id);
CREATE INDEX IF NOT EXISTS committee_members_invitation_idx ON committee_members (wedding_id, invitation_id);

-- invitation_id is deliberately not a foreign key: a guest or committee member is
-- materialized on acceptance but other invitation-scoped rows may reference an
-- invitation that is no longer part of the aggregate.
CREATE TABLE IF NOT EXISTS invitations (
    id              text PRIMARY KEY,
    wedding_id      text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    invitation_type text NOT NULL DEFAULT 'guest',
    guest_name      text NOT NULL DEFAULT '',
    guest_email     text NOT NULL DEFAULT '',
    guest_phone     text NOT NULL DEFAULT '',
    committee_title text NOT NULL DEFAULT '',
    max_party_size  integer NOT NULL DEFAULT 1,
    status          text NOT NULL,
    token_hash      text NOT NULL UNIQUE,
    expires_at      timestamptz,
    created_at      timestamptz NOT NULL,
    responded_at    timestamptz,
    position        integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS invitations_wedding_idx ON invitations (wedding_id, position, id);

CREATE TABLE IF NOT EXISTS events (
    id          text PRIMARY KEY,
    wedding_id  text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    name        text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz,
    venue       text NOT NULL DEFAULT '',
    address     text NOT NULL DEFAULT '',
    status      text NOT NULL,
    position    integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS events_wedding_idx ON events (wedding_id, position, id);

CREATE TABLE IF NOT EXISTS photos (
    id         text PRIMARY KEY,
    wedding_id text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    url        text NOT NULL DEFAULT '',
    alt_text   text NOT NULL DEFAULT '',
    caption    text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    status     text NOT NULL
);
CREATE INDEX IF NOT EXISTS photos_wedding_idx ON photos (wedding_id, sort_order, id);

CREATE TABLE IF NOT EXISTS story_sections (
    id         text PRIMARY KEY,
    wedding_id text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    title      text NOT NULL DEFAULT '',
    body       text NOT NULL DEFAULT '',
    photo_url  text NOT NULL DEFAULT '',
    sort_order integer NOT NULL DEFAULT 0,
    status     text NOT NULL
);
CREATE INDEX IF NOT EXISTS story_sections_wedding_idx ON story_sections (wedding_id, sort_order, id);

CREATE TABLE IF NOT EXISTS announcements (
    id           text PRIMARY KEY,
    wedding_id   text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    title        text NOT NULL DEFAULT '',
    body         text NOT NULL DEFAULT '',
    audience     text NOT NULL DEFAULT 'public',
    published_at timestamptz,
    status       text NOT NULL,
    author_name  text NOT NULL DEFAULT '',
    created_at   timestamptz,
    position     integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS announcements_wedding_idx ON announcements (wedding_id, position, id);

CREATE TABLE IF NOT EXISTS planning_tasks (
    id          text PRIMARY KEY,
    wedding_id  text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    title       text NOT NULL DEFAULT '',
    details     text NOT NULL DEFAULT '',
    assigned_to text NOT NULL DEFAULT '',
    due_on      text NOT NULL DEFAULT '',
    status      text NOT NULL,
    created_by  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    position    integer NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS planning_tasks_wedding_idx ON planning_tasks (wedding_id, position, id);

CREATE TABLE IF NOT EXISTS committee_messages (
    id          text PRIMARY KEY,
    wedding_id  text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    author_id   text NOT NULL DEFAULT '',
    author_name text NOT NULL DEFAULT '',
    author_role text NOT NULL DEFAULT '',
    body        text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS committee_messages_wedding_idx ON committee_messages (wedding_id, created_at, id);

CREATE TABLE IF NOT EXISTS rsvps (
    invitation_id text PRIMARY KEY,
    wedding_id    text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    status        text NOT NULL,
    party_size    integer NOT NULL DEFAULT 0,
    dietary_notes text NOT NULL DEFAULT '',
    updated_at    timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS rsvps_wedding_idx ON rsvps (wedding_id);

CREATE TABLE IF NOT EXISTS guest_messages (
    id            text PRIMARY KEY,
    wedding_id    text NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    invitation_id text NOT NULL DEFAULT '',
    body          text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS guest_messages_wedding_idx ON guest_messages (wedding_id, created_at, id);
