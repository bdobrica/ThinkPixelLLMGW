package ratelimit

import "context"

// Limiter is used to enforce per-key rate limits.
type Limiter interface {
	Allow(ctx context.Context, key string) bool
}

// NoopLimiter allows all requests (no rate limiting yet).
type NoopLimiter struct{}

func NewNoopLimiter() *NoopLimiter {
	return &NoopLimiter{}
}

func (l *NoopLimiter) Allow(ctx context.Context, key string) bool {
	// TODO: implement Redis-based rate limiting
	return true
}
