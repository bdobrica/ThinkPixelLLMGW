package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"llm_gateway/internal/utils"
)

type mockBuffer struct {
	records       []*LogRecord
	peekCalls     int
	dequeueCalls  int
	dequeueCounts []int
	dequeueErr    error
}

func (m *mockBuffer) Enqueue(ctx context.Context, record *LogRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockBuffer) Peek(ctx context.Context, count int) ([]*LogRecord, error) {
	m.peekCalls++
	if count > len(m.records) {
		count = len(m.records)
	}
	if count <= 0 {
		return nil, nil
	}
	out := make([]*LogRecord, count)
	copy(out, m.records[:count])
	return out, nil
}

func (m *mockBuffer) Dequeue(ctx context.Context, count int) ([]*LogRecord, error) {
	m.dequeueCalls++
	m.dequeueCounts = append(m.dequeueCounts, count)
	if m.dequeueErr != nil {
		return nil, m.dequeueErr
	}
	if count > len(m.records) {
		count = len(m.records)
	}
	removed := make([]*LogRecord, count)
	copy(removed, m.records[:count])
	m.records = m.records[count:]
	return removed, nil
}

func (m *mockBuffer) Size(ctx context.Context) (int64, error) {
	return int64(len(m.records)), nil
}

type mockBatchWriter struct {
	err        error
	writeCalls int
}

func (m *mockBatchWriter) WriteBatch(ctx context.Context, records []*LogRecord) (string, error) {
	m.writeCalls++
	if m.err != nil {
		return "", m.err
	}
	return "logs/test.jsonl.gz", nil
}

func TestNoopSink(t *testing.T) {
	sink := NewNoopSink()

	rec := &LogRecord{
		Timestamp: time.Now(),
		RequestID: "test-123",
		APIKeyID:  "key-456",
		Provider:  "openai",
		Model:     "gpt-4",
		CostUSD:   0.05,
	}

	err := sink.Enqueue(rec)
	if err != nil {
		t.Errorf("Expected no error from NoopSink.Enqueue, got %v", err)
	}

	err = sink.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected no error from NoopSink.Shutdown, got %v", err)
	}
}

func TestS3SinkConfig(t *testing.T) {
	config := S3SinkConfig{
		BufferSize:    1000,
		FlushSize:     100,
		FlushInterval: 5 * time.Minute,
		S3Bucket:      "test-bucket",
		S3Region:      "us-east-1",
		S3Prefix:      "logs/",
		PodName:       "test-pod",
	}

	if config.BufferSize != 1000 {
		t.Errorf("Expected buffer size 1000, got %d", config.BufferSize)
	}

	if config.FlushSize != 100 {
		t.Errorf("Expected flush size 100, got %d", config.FlushSize)
	}

	if config.S3Bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got '%s'", config.S3Bucket)
	}
}

func TestS3SinkFlush_DoesNotDequeueOnWriteFailure(t *testing.T) {
	buf := &mockBuffer{records: []*LogRecord{{RequestID: "req-1"}, {RequestID: "req-2"}}}
	writer := &mockBatchWriter{err: errors.New("s3 write failed")}

	sink := &S3Sink{
		buffer:    buf,
		writer:    writer,
		flushSize: 10,
		logger:    nil,
	}
	// logger is only used for side-effect logs; keep initialized for method safety.
	sink.logger = sinkLoggerForTest()

	sink.flush(context.Background())

	if writer.writeCalls != 1 {
		t.Fatalf("expected one write call, got %d", writer.writeCalls)
	}
	if buf.dequeueCalls != 0 {
		t.Fatalf("expected zero dequeue calls on write failure, got %d", buf.dequeueCalls)
	}
	if len(buf.records) != 2 {
		t.Fatalf("expected records to remain in buffer, got %d", len(buf.records))
	}
}

func TestS3SinkFlush_DequeueAfterSuccessfulWrite(t *testing.T) {
	buf := &mockBuffer{records: []*LogRecord{{RequestID: "req-1"}, {RequestID: "req-2"}, {RequestID: "req-3"}}}
	writer := &mockBatchWriter{}

	sink := &S3Sink{
		buffer:    buf,
		writer:    writer,
		flushSize: 2,
		logger:    sinkLoggerForTest(),
	}

	sink.flush(context.Background())

	if writer.writeCalls != 1 {
		t.Fatalf("expected one write call, got %d", writer.writeCalls)
	}
	if buf.dequeueCalls != 1 {
		t.Fatalf("expected one dequeue call, got %d", buf.dequeueCalls)
	}
	if len(buf.dequeueCounts) != 1 || buf.dequeueCounts[0] != 2 {
		t.Fatalf("expected dequeue count 2, got %v", buf.dequeueCounts)
	}
	if len(buf.records) != 1 {
		t.Fatalf("expected one record remaining, got %d", len(buf.records))
	}
}

func TestS3SinkFlushAll_StopsOnWriteFailureWithoutDropping(t *testing.T) {
	buf := &mockBuffer{records: []*LogRecord{{RequestID: "req-1"}, {RequestID: "req-2"}}}
	writer := &mockBatchWriter{err: errors.New("s3 write failed")}

	sink := &S3Sink{
		buffer:    buf,
		writer:    writer,
		flushSize: 10,
		logger:    sinkLoggerForTest(),
	}

	sink.flushAll(context.Background())

	if buf.dequeueCalls != 0 {
		t.Fatalf("expected no dequeue on failed flushAll write, got %d", buf.dequeueCalls)
	}
	if len(buf.records) != 2 {
		t.Fatalf("expected records to remain after failed flushAll, got %d", len(buf.records))
	}
}

func sinkLoggerForTest() *utils.Logger {
	return utils.NewLogger("s3-sink-test", utils.Error)
}

// Note: Full integration tests for S3Sink require AWS credentials and actual S3 bucket
// These should be run separately with appropriate environment setup
