package models

import "time"

// Operator binds a Keycloak identity (subject) to a SIP extension. It is the
// bridge between app-login identity and telephony identity for the webphone.
// RBAC roles are NOT stored here — they come from the Keycloak JWT.
type Operator struct {
	ID          string    `json:"id"`
	Subject     string    `json:"subject"` // Keycloak `sub` (or username)
	Domain      string    `json:"domain"`
	Number      string    `json:"number"` // SIP extension
	DisplayName string    `json:"display_name,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
