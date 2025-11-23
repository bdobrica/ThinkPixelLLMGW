package billing

import "context"

// Service tracks costs and enforces budgets.
type Service interface {
	WithinBudget(ctx context.Context, apiKeyID string) bool
	AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error
}

// NoopService does not enforce budgets and discards usage.
type NoopService struct{}

func NewNoopService() *NoopService {
	return &NoopService{}
}

func (s *NoopService) WithinBudget(ctx context.Context, apiKeyID string) bool {
	// TODO: implement budget checks using Redis + DB
	return true
}

func (s *NoopService) AddUsage(ctx context.Context, apiKeyID string, costUSD float64) error {
	// TODO: accumulate running cost in Redis and periodically persist to DB
	return nil
}
