package models

import (
	"encoding/json"
	"time"
)

// AuditLog is a single change record. Before/After are the redacted JSON
// snapshots stored by the audit recorder (raw JSON, may be null).
type AuditLog struct {
	ID           string          `json:"id"`
	Actor        string          `json:"actor"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Before       json.RawMessage `json:"before,omitempty"`
	After        json.RawMessage `json:"after,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// AuditFilter narrows an audit query. Empty string fields are ignored.
type AuditFilter struct {
	Actor        string
	Action       string
	ResourceType string
	ResourceID   string
	Limit        int
	Offset       int
}
