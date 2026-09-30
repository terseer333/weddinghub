-- Platform admin audit trail.
--
-- Entries are append-only: the dashboard reads them and never edits or deletes them, so there is
-- no update path in the repository. actor_id intentionally has no foreign key to users: the trail
-- must survive an account being deleted, which is itself an auditable action. metadata holds small
-- JSON details such as the client address or the identifier of an affected resource.
CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id            text PRIMARY KEY,
    actor_id      text NOT NULL DEFAULT '',
    actor_email   text NOT NULL DEFAULT '',
    action        text NOT NULL,
    resource_type text NOT NULL DEFAULT '',
    resource_id   text NOT NULL DEFAULT '',
    metadata      jsonb,
    created_at    timestamptz NOT NULL
);
-- The dashboard lists newest first, so the ordering index carries created_at DESC.
CREATE INDEX IF NOT EXISTS admin_audit_logs_created_idx ON admin_audit_logs (created_at DESC, id);
CREATE INDEX IF NOT EXISTS admin_audit_logs_actor_idx ON admin_audit_logs (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS admin_audit_logs_resource_idx ON admin_audit_logs (resource_type, resource_id);
