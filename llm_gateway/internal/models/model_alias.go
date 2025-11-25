package models

import (
	"time"

	"github.com/google/uuid"
)

// ModelAlias maps a public model alias to a concrete provider/model pair.
type ModelAlias struct {
	ID            uuid.UUID `db:"id"`
	Alias         string    `db:"alias"`
	TargetModelID uuid.UUID `db:"target_model_id"`
	ProviderID    uuid.UUID `db:"provider_id"`
	CustomConfig  JSONB     `db:"custom_config"`
	Enabled       bool      `db:"enabled"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`

	// Not stored in DB, populated in code
	Tags map[string]string `db:"-"`
}
