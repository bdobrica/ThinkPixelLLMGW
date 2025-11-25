package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

//
// JSONB helper
//

// JSONB is a helper for Postgres jsonb columns.
// Backed by map[string]any and works with sqlx / database/sql.
type JSONB map[string]any

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("JSONB: expected []byte, got %T", value)
	}

	if len(b) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(b, j)
}
