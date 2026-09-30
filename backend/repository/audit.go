package repository

import (
	"encoding/json"

	"weddinghub/models"
)

const (
	// defaultAuditLogLimit caps a listing when the caller does not ask for a size.
	defaultAuditLogLimit = 200
	// maxAuditLogs bounds the in-memory audit trail used by tests and local runs.
	maxAuditLogs = 5000
)

// cloneAuditLog copies an entry, including its metadata map, so a caller cannot mutate stored
// state through the returned value.
func cloneAuditLog(entry models.AuditLog) models.AuditLog {
	if entry.Metadata != nil {
		metadata := make(map[string]string, len(entry.Metadata))
		for key, value := range entry.Metadata {
			metadata[key] = value
		}
		entry.Metadata = metadata
	}
	return entry
}

// auditMetadataValue renders metadata for a jsonb column. Following the card_config convention,
// JSON is sent as a string with an explicit ::jsonb cast — a []byte parameter would be sent as
// bytea and rejected by PostgreSQL. nil becomes SQL NULL.
func auditMetadataValue(metadata map[string]string) any {
	if len(metadata) == 0 {
		return nil
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil
	}
	return string(encoded)
}

func (r *PostgresRepository) AddAuditLog(entry models.AuditLog) error {
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.db.ExecContext(ctx, `INSERT INTO admin_audit_logs
		(id, actor_id, actor_email, action, resource_type, resource_id, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8)`,
		entry.ID, entry.ActorID, entry.ActorEmail, entry.Action, entry.ResourceType, entry.ResourceID,
		auditMetadataValue(entry.Metadata), entry.CreatedAt)
	return err
}

func (r *PostgresRepository) ListAuditLogs(limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = defaultAuditLogLimit
	}
	ctx, cancel := r.ctx()
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT id, actor_id, actor_email, action, resource_type, resource_id, metadata, created_at
		FROM admin_audit_logs ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AuditLog, 0, limit)
	for rows.Next() {
		var (
			entry    models.AuditLog
			metadata []byte
		)
		if err := rows.Scan(&entry.ID, &entry.ActorID, &entry.ActorEmail, &entry.Action, &entry.ResourceType,
			&entry.ResourceID, &metadata, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if len(metadata) > 0 {
			if err := json.Unmarshal(metadata, &entry.Metadata); err != nil {
				return nil, err
			}
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}
