package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Recorder struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Recorder {
	return &Recorder{pool: pool}
}

// Log writes an audit entry. Sensitive keys are redacted in before/after.
// Failures are intentionally swallowed: auditing must never break a request.
func (r *Recorder) Log(ctx context.Context, actor, action, resourceType, resourceID string, before, after any) {
	if r == nil || r.pool == nil {
		return
	}
	beforeJSON := redact(before)
	afterJSON := redact(after)
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, actor, action, resource_type, resource_id, before, after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.NewString(), actor, action, resourceType, resourceID, beforeJSON, afterJSON)
}

var sensitiveKeys = map[string]bool{
	"password":    true,
	"vm-password": true,
}

func redact(v any) []byte {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		// Not an object; return as-is.
		return raw
	}
	redactMap(m)
	out, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return out
}

func redactMap(m map[string]any) {
	for k, v := range m {
		if sensitiveKeys[k] {
			m[k] = "***"
			continue
		}
		switch child := v.(type) {
		case map[string]any:
			redactMap(child)
		}
	}
}
