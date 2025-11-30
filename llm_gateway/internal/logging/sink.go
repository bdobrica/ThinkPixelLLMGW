package logging

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"llm_gateway/internal/queue"
	"llm_gateway/internal/utils"
)

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

// S3Sink buffers log records in-memory and flushes to S3 periodically or when buffer fills
type S3Sink struct {
	queue         queue.Queue
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

// NewS3Sink creates a new S3-based logging sink
func NewS3Sink(ctx context.Context, config S3SinkConfig) (*S3Sink, error) {
	// Create S3 writer
	writer, err := NewS3Writer(ctx, config.S3Bucket, config.S3Region, config.S3Prefix, config.PodName)
	if err != nil {
		return nil, err
	}

	// Create in-memory queue (no Redis needed for logging)
	queueConfig := &queue.Config{
		QueueName:    "logging",
		BatchSize:    config.FlushSize,
		BatchTimeout: config.FlushInterval,
		MaxRetries:   0, // No retries for logging
		RetryBackoff: 0,
		UseRedis:     false,
	}
	memQueue := queue.NewMemoryQueue(queueConfig)

	sink := &S3Sink{
		queue:         memQueue,
		writer:        writer,
		flushSize:     config.FlushSize,
		flushInterval: config.FlushInterval,
		logger:        utils.NewLogger("s3-sink"),
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
	}

	// Start background flusher
	sink.wg.Add(1)
	go sink.run(ctx)

	// Setup signal handlers for graceful shutdown
	sink.setupSignalHandlers()

	return sink, nil
}

// S3SinkConfig holds configuration for S3Sink
type S3SinkConfig struct {
	BufferSize    int
	FlushSize     int
	FlushInterval time.Duration
	S3Bucket      string
	S3Region      string
	S3Prefix      string
	PodName       string
}

// Enqueue adds a log record to the in-memory queue
func (s *S3Sink) Enqueue(rec *LogRecord) error {
	ctx := context.Background()
	return s.queue.Enqueue(ctx, rec)
}

// run is the main worker loop that flushes batches to S3
func (s *S3Sink) run(ctx context.Context) {
	defer s.wg.Done()
	defer close(s.stoppedChan)

	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	// Also create a ticker for checking flush size
	sizeTicker := time.NewTicker(100 * time.Millisecond)
	defer sizeTicker.Stop()

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
			s.flush(ctx)
		case <-sizeTicker.C:
			// Check if we've reached flush size
			length, err := s.queue.Length(ctx)
			if err != nil {
				s.logger.Error("Failed to get queue length", "error", err)
				continue
			}

			if length >= s.flushSize {
				s.flush(ctx)
			}
		}
	}
}

// flush writes a batch of records to S3
func (s *S3Sink) flush(ctx context.Context) {
	// Dequeue up to flushSize items with short timeout
	items, err := s.queue.DequeueWithTimeout(ctx, s.flushSize, 100*time.Millisecond)
	if err != nil {
		s.logger.Error("Failed to dequeue records", "error", err)
		return
	}

	if len(items) == 0 {
		return
	}

	// Convert items to LogRecords
	records := make([]*LogRecord, 0, len(items))
	for _, item := range items {
		if rec, ok := item.(*LogRecord); ok {
			records = append(records, rec)
		} else {
			s.logger.Warn("Invalid item type in queue", "type", fmt.Sprintf("%T", item))
		}
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

	s.logger.Debug("Flushed batch to S3", "key", key, "count", len(records))
}

// flushAll drains the entire queue and writes to S3
func (s *S3Sink) flushAll(ctx context.Context) {
	totalFlushed := 0
	for {
		items, err := s.queue.DequeueWithTimeout(ctx, s.flushSize, 100*time.Millisecond)
		if err != nil || len(items) == 0 {
			break
		}

		records := make([]*LogRecord, 0, len(items))
		for _, item := range items {
			if rec, ok := item.(*LogRecord); ok {
				records = append(records, rec)
			}
		}

		if len(records) > 0 {
			_, err := s.writer.WriteBatch(ctx, records)
			if err != nil {
				s.logger.Error("Failed to write final batch to S3", "error", err)
			} else {
				totalFlushed += len(records)
			}
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

// Shutdown gracefully stops the sink and flushes remaining records
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
func NewSinkFromConfig(ctx context.Context, config S3SinkConfig) (Sink, error) {
	if config.S3Bucket == "" {
		return NewNoopSink(), nil
	}

	return NewS3Sink(ctx, config)
}
