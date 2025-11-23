package models

import "time"

// ModelAlias maps a public model alias to a concrete provider/model pair.
type ModelAlias struct {
	ID            string
	Alias         string // e.g. "project1-gpt-5"
	ProviderID    string
	ProviderModel string // e.g. "gpt-5"
	BillingKeyID  *string
	CreatedAt     time.Time
}
