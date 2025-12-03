package logging

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"llm_gateway/internal/utils"
)

// LogBuffer defines the interface for buffering log records
type LogBuffer interface {
	Enqueue(ctx context.Context, record *LogRecord) error
	Dequeue(ctx context.Context, count int) ([]*LogRecord, error)
	Size(ctx context.Context) (int64, error)
}

// LogRecord is the structure that will be logged to S3 via in-memory buffering.
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
	Shutdown(ctx context.Context) error
}

// NoopSink is a placeholder implementation that discards logs.
type NoopSink struct{}

func NewNoopSink() *NoopSink {
	return &NoopSink{}
}

func (s *NoopSink) Enqueue(rec *LogRecord) error {
	return nil
}

func (s *NoopSink) Shutdown(ctx context.Context) error {
	return nil
}

// S3Sink drains log records from Redis buffer and flushes to S3 periodically
type S3Sink struct {
	buffer        LogBuffer
	writer        *S3Writer
	flushSize     int
	flushInterval time.Duration
	logger        *utils.Logger

	stopChan    chan struct{}
	stoppedChan chan struct{}
	wg          sync.WaitGroup

	// NOTE: Remember to configure Kubernetes preStop hooks to allow enough time
	// for graceful shutdown and buffer flushing. Recommended: 30-60 seconds.
	// Example preStop hook:
	//   lifecycle:
	//     preStop:
	//       exec:
	//         command: ["/bin/sh", "-c", "sleep 30"]
}

// NewS3Sink creates a new S3-based logging sink with Redis buffer
func NewS3Sink(ctx context.Context, config S3SinkConfig, buffer LogBuffer) (*S3Sink, error) {
	// Create S3 writer
	writer, err := NewS3Writer(ctx, config.S3Bucket, config.S3Region, config.S3Prefix, config.PodName)
	if err != nil {
		return nil, err
	}

	sink := &S3Sink{
		buffer:        buffer,
		writer:        writer,
		flushSize:     config.FlushSize,
		flushInterval: config.FlushInterval,
		logger:        utils.NewLogger("s3-sink", utils.Info),
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
	}

	// Start background worker that drains Redis buffer to S3
	sink.logger.Info("Starting S3 sink background worker")
	sink.wg.Add(1)
	go sink.run(ctx)

	return sink, nil
}

// S3SinkConfig holds configuration for S3Sink
type S3SinkConfig struct {
	Enabled       bool
	BufferSize    int
	FlushSize     int
	FlushInterval time.Duration
	S3Bucket      string
	S3Region      string
	S3Prefix      string
	PodName       string
}

// Enqueue adds a log record to the Redis buffer
func (s *S3Sink) Enqueue(rec *LogRecord) error {
	ctx := context.Background()
	return s.buffer.Enqueue(ctx, rec)
}

// run is the main background worker loop that drains Redis buffer and flushes batches to S3
func (s *S3Sink) run(ctx context.Context) {
	defer s.wg.Done()
	defer close(s.stoppedChan)

	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	// Also create a ticker for checking flush size
	sizeTicker := time.NewTicker(1 * time.Second)
	defer sizeTicker.Stop()

	s.logger.Info("S3 background worker started",
		"flush_interval", s.flushInterval,
		"flush_size", s.flushSize,
	)

	for {
		select {
		case <-s.stopChan:
			s.logger.Info("S3 sink stopping, flushing remaining records")
			s.flushAll(ctx)
			return
		case <-ctx.Done():
			s.logger.Info("S3 sink context cancelled, flushing remaining records")
			s.flushAll(ctx)
			return
		case <-ticker.C:
			// Periodic flush
			s.flush(ctx)
		case <-sizeTicker.C:
			// Check if we've reached flush size
			size, err := s.buffer.Size(ctx)
			if err != nil {
				s.logger.Error("Failed to get buffer size", "error", err)
				continue
			}

			if size >= int64(s.flushSize) {
				s.flush(ctx)
			}
		}
	}
}

// flush writes a batch of records from Redis buffer to S3
func (s *S3Sink) flush(ctx context.Context) {
	// Dequeue up to flushSize items from Redis
	records, err := s.buffer.Dequeue(ctx, s.flushSize)
	if err != nil {
		s.logger.Error("Failed to dequeue records from Redis", "error", err)
		return
	}

	if len(records) == 0 {
		return
	}

	// Write to S3
	key, err := s.writer.WriteBatch(ctx, records)
	if err != nil {
		s.logger.Error("Failed to write batch to S3", "error", err, "count", len(records))
		// Note: Records are lost on failure. Consider adding DLQ if needed.
		return
	}

	s.logger.Info("Flushed batch to S3", "key", key, "count", len(records))
}

// flushAll drains the entire Redis buffer and writes to S3
func (s *S3Sink) flushAll(ctx context.Context) {
	totalFlushed := 0
	for {
		records, err := s.buffer.Dequeue(ctx, s.flushSize)
		if err != nil || len(records) == 0 {
			break
		}

		_, err = s.writer.WriteBatch(ctx, records)
		if err != nil {
			s.logger.Error("Failed to write final batch to S3", "error", err)
		} else {
			totalFlushed += len(records)
		}
	}

	if totalFlushed > 0 {
		s.logger.Info("Flushed remaining records on shutdown", "count", totalFlushed)
	}
}

// setupSignalHandlers configures signal handlers for graceful shutdown
func (s *S3Sink) setupSignalHandlers() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		s.logger.Info("Received signal, initiating graceful shutdown", "signal", sig.String())
		// Give ourselves time to flush
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
}

// Shutdown gracefully stops the sink and flushes remaining records from Redis to S3
func (s *S3Sink) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down S3 sink")
	close(s.stopChan)

	// Wait for worker to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("S3 sink shutdown complete")
		return nil
	case <-ctx.Done():
		s.logger.Warn("S3 sink shutdown timed out")
		return ctx.Err()
	}
}

// NewSinkFromConfig creates the appropriate sink based on configuration
func NewSinkFromConfig(ctx context.Context, config S3SinkConfig, buffer LogBuffer) (Sink, error) {
	logger := utils.NewLogger("sink-factory", utils.Info)

	if !config.Enabled {
		logger.Info("S3 logging sink disabled (LOGGING_SINK_ENABLED=false)")
		return NewNoopSink(), nil
	}

	if config.S3Bucket == "" {
		logger.Warn("S3 logging sink disabled (LOGGING_SINK_S3_BUCKET not set)")
		return NewNoopSink(), nil
	}

	logger.Info("Initializing S3 logging sink",
		"bucket", config.S3Bucket,
		"region", config.S3Region,
		"prefix", config.S3Prefix,
		"flushInterval", config.FlushInterval,
		"flushSize", config.FlushSize,
	)

	return NewS3Sink(ctx, config, buffer)
}
