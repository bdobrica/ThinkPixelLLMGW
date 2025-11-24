package storage

import "errors"

var (
	// ErrAPIKeyNotFound is returned when an API key is not found
	ErrAPIKeyNotFound = errors.New("API key not found")

	// ErrModelNotFound is returned when a model is not found
	ErrModelNotFound = errors.New("model not found")

	// ErrModelAliasNotFound is returned when a model alias is not found
	ErrModelAliasNotFound = errors.New("model alias not found")

	// ErrProviderNotFound is returned when a provider is not found
	ErrProviderNotFound = errors.New("provider not found")

	// ErrUsageRecordNotFound is returned when a usage record is not found
	ErrUsageRecordNotFound = errors.New("usage record not found")
)
