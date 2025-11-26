package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondWithError(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
	}{
		{
			name:    "bad request",
			code:    http.StatusBadRequest,
			message: "Invalid input",
		},
		{
			name:    "unauthorized",
			code:    http.StatusUnauthorized,
			message: "Authentication required",
		},
		{
			name:    "not found",
			code:    http.StatusNotFound,
			message: "Resource not found",
		},
		{
			name:    "internal server error",
			code:    http.StatusInternalServerError,
			message: "Something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response recorder
			w := httptest.NewRecorder()

			// Call the function
			RespondWithError(w, tt.code, tt.message)

			// Check status code
			if w.Code != tt.code {
				t.Errorf("RespondWithError() status = %d, want %d", w.Code, tt.code)
			}

			// Check content type
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("RespondWithError() Content-Type = %s, want application/json", contentType)
			}

			// Parse response body
			var response ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Check error message
			if response.Error != tt.message {
				t.Errorf("RespondWithError() message = %s, want %s", response.Error, tt.message)
			}
		})
	}
}

func TestRespondWithJSON(t *testing.T) {
	t.Run("simple struct", func(t *testing.T) {
		w := httptest.NewRecorder()

		payload := struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}{
			Name:  "John Doe",
			Age:   30,
			Email: "john@example.com",
		}

		err := RespondWithJSON(w, http.StatusOK, payload)
		if err != nil {
			t.Errorf("RespondWithJSON() error = %v, want nil", err)
		}

		if w.Code != http.StatusOK {
			t.Errorf("RespondWithJSON() status = %d, want %d", w.Code, http.StatusOK)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("RespondWithJSON() Content-Type = %s, want application/json", contentType)
		}

		var response struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email"`
		}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Name != payload.Name {
			t.Errorf("RespondWithJSON() name = %s, want %s", response.Name, payload.Name)
		}
		if response.Age != payload.Age {
			t.Errorf("RespondWithJSON() age = %d, want %d", response.Age, payload.Age)
		}
	})

	t.Run("map payload", func(t *testing.T) {
		w := httptest.NewRecorder()

		payload := map[string]any{
			"success": true,
			"count":   42,
			"items":   []string{"a", "b", "c"},
		}

		err := RespondWithJSON(w, http.StatusCreated, payload)
		if err != nil {
			t.Errorf("RespondWithJSON() error = %v, want nil", err)
		}

		if w.Code != http.StatusCreated {
			t.Errorf("RespondWithJSON() status = %d, want %d", w.Code, http.StatusCreated)
		}

		var response map[string]any
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["success"] != true {
			t.Errorf("RespondWithJSON() success = %v, want true", response["success"])
		}
		if int(response["count"].(float64)) != 42 {
			t.Errorf("RespondWithJSON() count = %v, want 42", response["count"])
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		w := httptest.NewRecorder()

		payload := struct{}{}

		err := RespondWithJSON(w, http.StatusNoContent, payload)
		if err != nil {
			t.Errorf("RespondWithJSON() error = %v, want nil", err)
		}

		if w.Code != http.StatusNoContent {
			t.Errorf("RespondWithJSON() status = %d, want %d", w.Code, http.StatusNoContent)
		}
	})

	t.Run("nil payload", func(t *testing.T) {
		w := httptest.NewRecorder()

		err := RespondWithJSON(w, http.StatusOK, nil)
		if err != nil {
			t.Errorf("RespondWithJSON() error = %v, want nil", err)
		}

		body := w.Body.String()
		if body != "null\n" {
			t.Logf("RespondWithJSON() with nil payload body = %q", body)
		}
	})
}

func TestErrorResponse(t *testing.T) {
	err := ErrorResponse{Error: "test error"}

	if err.Error != "test error" {
		t.Errorf("ErrorResponse.Error = %s, want test error", err.Error)
	}

	// Test JSON marshaling
	data, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		t.Fatalf("Failed to marshal ErrorResponse: %v", jsonErr)
	}

	var decoded ErrorResponse
	if jsonErr := json.Unmarshal(data, &decoded); jsonErr != nil {
		t.Fatalf("Failed to unmarshal ErrorResponse: %v", jsonErr)
	}

	if decoded.Error != err.Error {
		t.Errorf("Decoded error = %s, want %s", decoded.Error, err.Error)
	}
}

func TestRespondWithErrorIntegration(t *testing.T) {
	// Test the common error response patterns
	testCases := []struct {
		name           string
		code           int
		message        string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "validation error",
			code:           400,
			message:        "Field 'email' is required",
			expectedStatus: 400,
			expectedError:  "Field 'email' is required",
		},
		{
			name:           "api key error",
			code:           401,
			message:        "Invalid API key",
			expectedStatus: 401,
			expectedError:  "Invalid API key",
		},
		{
			name:           "rate limit error",
			code:           429,
			message:        "Rate limit exceeded",
			expectedStatus: 429,
			expectedError:  "Rate limit exceeded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondWithError(w, tc.code, tc.message)

			if w.Code != tc.expectedStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tc.expectedStatus)
			}

			var resp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if resp.Error != tc.expectedError {
				t.Errorf("Error message = %s, want %s", resp.Error, tc.expectedError)
			}
		})
	}
}
