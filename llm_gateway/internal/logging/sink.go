package logging

import "time"

// Record is the structure that will be logged to S3 (via Redis buffering).
type LogRecord struct {
	Timestamp  time.Time         `json:"timestamp"`
	RequestID  string            `json:"request_id"`
	APIKeyID   string            `json:"api_key_id"`
	APIKeyName string            `json:"api_key_name,omitempty"`
	Provider   string            `json:"provider"`
	Model      string            `json:"model"`
	Alias      string            `json:"alias,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	ProviderMs int64             `json:"provider_ms"`
	GatewayMs  int64             `json:"gateway_ms"`
	CostUSD    float64           `json:"cost_usd"`
	Error      string            `json:"error,omitempty"`
	// For now we keep request/response opaque; you can refine later.
	RequestPayload  any `json:"request_payload,omitempty"`
	ResponsePayload any `json:"response_payload,omitempty"`
}

// Sink receives log records from the gateway.
type Sink interface {
	Enqueue(rec *LogRecord) error
}

// NoopSink is a placeholder implementation that discards logs.
type NoopSink struct{}

func NewNoopSink() *NoopSink {
	return &NoopSink{}
}

func (s *NoopSink) Enqueue(rec *LogRecord) error {
	// TODO: implement Redis buffer → S3 writer
	return nil
}
