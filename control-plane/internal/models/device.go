package models

import "time"

// Device is a provisioned physical SIP phone.
type Device struct {
	ID          string    `json:"id"`
	MAC         string    `json:"mac"`
	Vendor      string    `json:"vendor"`
	Model       string    `json:"model,omitempty"`
	Number      string    `json:"number"`
	Domain      string    `json:"domain"`
	DisplayName string    `json:"display_name,omitempty"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DeviceAccount is the resolved data needed to render a provisioning config:
// the device plus the SIP secret pulled from its directory user.
type DeviceAccount struct {
	Device
	Password string
}
