package logging

// Record is the structure that will be logged to S3 (via Redis buffering).
type Record struct {
	APIKeyID string            `json:"api_key_id"`
	Provider string            `json:"provider"`
	Model    string            `json:"model"`
	Tags     map[string]string `json:"tags,omitempty"`
	// TODO: add latency, cost, request/response, error, etc.
}

// Sink receives log records from the gateway.
type Sink interface {
	Enqueue(rec *Record) error
}

// NoopSink is a placeholder implementation that discards logs.
type NoopSink struct{}

func NewNoopSink() *NoopSink {
	return &NoopSink{}
}

func (s *NoopSink) Enqueue(rec *Record) error {
	// TODO: implement Redis buffer → S3 writer
	return nil
}
