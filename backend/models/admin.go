package models

import "time"

// Actions recorded in the admin audit trail. They are stable identifiers, so a dashboard or an
// export can filter on them without depending on human-readable text.
const (
	AuditAdminLogin        = "admin.login"
	AuditAdminLoginFailed  = "admin.login_failed"
	AuditAdminLogout       = "admin.logout"
	AuditAdminAccessDenied = "admin.access_denied"
)

// AuditLog is one recorded administrative action. The trail is append-only: the admin interface
// only ever reads it, and actor_id deliberately has no foreign key so an entry survives the
// deletion of the account that produced it.
type AuditLog struct {
	ID           string            `json:"id"`
	ActorID      string            `json:"actor_id,omitempty"`
	ActorEmail   string            `json:"actor_email,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type,omitempty"`
	ResourceID   string            `json:"resource_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}
