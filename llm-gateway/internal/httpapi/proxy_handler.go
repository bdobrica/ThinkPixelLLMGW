package httpapi

import (
	"encoding/json"
	"net/http"
)

// handleChat is the entry point for OpenAI-compatible chat completions.
func (d *Dependencies) handleChat(w http.ResponseWriter, r *http.Request) {
	// TODO: parse Authorization header, look up key, rate limit, budget check,
	// resolve model alias, call provider, log & return response.

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "proxy not implemented yet",
	})
}
