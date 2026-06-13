package models

import "time"

type Domain struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Enabled     bool              `json:"enabled"`
	Variables   map[string]string `json:"variables"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type User struct {
	ID        string            `json:"id"`
	Domain    string            `json:"domain"`
	Number    string            `json:"number"`
	Enabled   bool              `json:"enabled"`
	Params    map[string]string `json:"params"`
	Variables map[string]string `json:"variables"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type Gateway struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Profile   string            `json:"profile"`
	Enabled   bool              `json:"enabled"`
	Username  string            `json:"username,omitempty"`
	Password  string            `json:"password,omitempty"`
	Realm     string            `json:"realm,omitempty"`
	Proxy     string            `json:"proxy"`
	Register  bool              `json:"register"`
	Params    map[string]string `json:"params"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type DialplanAction struct {
	Application string `json:"application"`
	Data        string `json:"data,omitempty"`
}

type DialplanCondition struct {
	Field      string           `json:"field"`
	Expression string           `json:"expression"`
	Actions    []DialplanAction `json:"actions"`
}

type DialplanExtension struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Domain     string              `json:"domain"`
	Context    string              `json:"context"`
	Priority   int                 `json:"priority"`
	Enabled    bool                `json:"enabled"`
	Conditions []DialplanCondition `json:"conditions"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// DomainWithUsers is the aggregate used by the directory renderer.
type DomainWithUsers struct {
	Domain Domain
	Users  []User
}
