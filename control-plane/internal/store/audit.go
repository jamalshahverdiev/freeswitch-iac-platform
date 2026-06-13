package store

import (
	"context"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

// ListAuditLogs returns audit entries newest-first, narrowed by the filter.
// A zero Limit means no limit. Also returns the total count matching the
// filter (ignoring limit/offset) for pagination headers.
func (s *Store) ListAuditLogs(ctx context.Context, f models.AuditFilter) ([]models.AuditLog, int, error) {
	where := `WHERE ($1='' OR actor=$1)
	            AND ($2='' OR action=$2)
	            AND ($3='' OR resource_type=$3)
	            AND ($4='' OR resource_id=$4)`
	args := []any{f.Actor, f.Action, f.ResourceType, f.ResourceID}

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `SELECT id, actor, action, resource_type, resource_id, before, after, created_at
	      FROM audit_logs ` + where + ` ORDER BY created_at DESC, id`
	if f.Limit > 0 {
		q += ` LIMIT $5 OFFSET $6`
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.AuditLog{}
	for rows.Next() {
		var a models.AuditLog
		if err := rows.Scan(&a.ID, &a.Actor, &a.Action, &a.ResourceType, &a.ResourceID,
			&a.Before, &a.After, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}
