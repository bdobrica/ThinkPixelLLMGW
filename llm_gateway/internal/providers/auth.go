package providers

import (
	"context"
	"fmt"
	"net/http"
)

// SimpleAPIKeyAuth implements API key authentication (OpenAI-style)
type SimpleAPIKeyAuth struct {
	apiKey     string
	headerName string // e.g., "Authorization"
	prefix     string // e.g., "Bearer "
}

// NewSimpleAPIKeyAuth creates a new simple API key authenticator
func NewSimpleAPIKeyAuth(apiKey, headerName, prefix string) *SimpleAPIKeyAuth {
	if headerName == "" {
		headerName = "Authorization"
	}
	if prefix == "" {
		prefix = "Bearer "
	}

	return &SimpleAPIKeyAuth{
		apiKey:     apiKey,
		headerName: headerName,
		prefix:     prefix,
	}
}

// Authenticate returns an auth context with the API key
func (a *SimpleAPIKeyAuth) Authenticate(ctx context.Context) (AuthContext, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	return &SimpleAPIKeyAuthContext{
		apiKey:     a.apiKey,
		headerName: a.headerName,
		prefix:     a.prefix,
	}, nil
}

// SimpleAPIKeyAuthContext holds the auth context for API key authentication
type SimpleAPIKeyAuthContext struct {
	apiKey     string
	headerName string
	prefix     string
}

// ApplyToRequest adds the API key to the HTTP request
func (c *SimpleAPIKeyAuthContext) ApplyToRequest(ctx context.Context, req any) error {
	httpReq, ok := req.(*http.Request)
	if !ok {
		return fmt.Errorf("expected *http.Request, got %T", req)
	}

	httpReq.Header.Set(c.headerName, c.prefix+c.apiKey)
	return nil
}
