package models

import "time"

// APIKey represents a client API key managed by the admin API.
type APIKey struct {
	ID                 string
	Name               string
	Hash               []byte
	AllowedModels      []string
	RateLimitPerMinute int
	MonthlyBudgetUSD   float64
	Tags               map[string]string
	Revoked            bool
	CreatedAt          time.Time
}

// AllowsModel checks if the key is allowed to call the given model (or alias).
func (k *APIKey) AllowsModel(model string) bool {
	for _, m := range k.AllowedModels {
		if m == model {
			return true
		}
	}
	return false
}
