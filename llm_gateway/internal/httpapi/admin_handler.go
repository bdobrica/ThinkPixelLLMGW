package httpapi

import (
	"encoding/json"
	"net/http"
)

// handleAdminKeys serves the key management API (create/revoke/regenerate).
func (d *Dependencies) handleAdminKeys(w http.ResponseWriter, r *http.Request) {
	// TODO: route by method, e.g. POST=create, GET=list, etc.

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "admin keys endpoint not implemented yet",
	})
}

// handleAdminProviders manages providers & model aliases.
func (d *Dependencies) handleAdminProviders(w http.ResponseWriter, r *http.Request) {
	// TODO: handle CRUD for providers and model aliases.

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "admin providers endpoint not implemented yet",
	})
}
