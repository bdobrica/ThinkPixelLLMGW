package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"llm_gateway/internal/queue"
	"llm_gateway/internal/utils"
)

// Integration tests for S3 logging sink using Minio
//
// To run these tests, start a Minio container:
//
//   docker run -d --name minio-test \
//     -p 9000:9000 -p 9001:9001 \
//     -e MINIO_ROOT_USER=minioadmin \
//     -e MINIO_ROOT_PASSWORD=minioadmin \
//     minio/minio server /data --console-address ":9001"
//
// Or use docker-compose:
//
//   version: '3'
//   services:
//     minio:
//       image: minio/minio
//       ports:
//         - "9000:9000"
//         - "9001:9001"
//       environment:
//         MINIO_ROOT_USER: minioadmin
//         MINIO_ROOT_PASSWORD: minioadmin
//       command: server /data --console-address ":9001"
//
// Then run tests:
//   MINIO_ENDPOINT=http://localhost:9000 go test -v -run TestS3Integration

const (
	defaultMinioEndpoint  = "http://localhost:9000"
	defaultMinioAccessKey = "minioadmin"
	defaultMinioSecretKey = "minioadmin"
	testBucketName        = "test-llm-logs"
)

// getMinioEndpoint returns the Minio endpoint from environment or default
func getMinioEndpoint() string {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = defaultMinioEndpoint
	}
	return endpoint
}

// getMinioCredentials returns access key and secret key from environment or defaults
func getMinioCredentials() (string, string) {
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = defaultMinioAccessKey
	}
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	if secretKey == "" {
		secretKey = defaultMinioSecretKey
	}
	return accessKey, secretKey
}

// isMinioAvailable checks if Minio is available for testing
func isMinioAvailable(t *testing.T) bool {
	client, err := createMinioClient()
	if err != nil {
		t.Skipf("Failed to create Minio client: %v", err)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to list buckets
	_, err = client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		t.Skipf("Minio not available for testing: %v", err)
		return false
	}
	return true
}

// createMinioClient creates an S3 client configured for Minio
func createMinioClient() (*s3.Client, error) {
	endpoint := getMinioEndpoint()
	accessKey, secretKey := getMinioCredentials()

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // Required for Minio
	})

	return client, nil
}

// setupTestBucket creates a test bucket if it doesn't exist
func setupTestBucket(t *testing.T, client *s3.Client) {
	ctx := context.Background()

	// Check if bucket exists
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(testBucketName),
	})
	if err == nil {
		return // Bucket already exists
	}

	// Create bucket
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucketName),
	})
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}
}

// cleanupTestBucket removes all objects from the test bucket
func cleanupTestBucket(t *testing.T, client *s3.Client) {
	ctx := context.Background()

	// List all objects
	listOutput, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(testBucketName),
	})
	if err != nil {
		t.Logf("Warning: failed to list objects: %v", err)
		return
	}

	// Delete all objects
	for _, obj := range listOutput.Contents {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(testBucketName),
			Key:    obj.Key,
		})
		if err != nil {
			t.Logf("Warning: failed to delete object %s: %v", *obj.Key, err)
		}
	}
}

// TestS3Integration_WriteBatch tests writing a batch of records to S3
func TestS3Integration_WriteBatch(t *testing.T) {
	if !isMinioAvailable(t) {
		return
	}

	client, err := createMinioClient()
	if err != nil {
		t.Fatalf("Failed to create Minio client: %v", err)
	}

	setupTestBucket(t, client)
	defer cleanupTestBucket(t, client)

	ctx := context.Background()

	// Create S3 writer
	writer := &S3Writer{
		client:  client,
		bucket:  testBucketName,
		prefix:  "test-logs/",
		podName: "test-pod",
		logger:  NewTestLogger(),
	}

	// Create test records
	records := []*LogRecord{
		{
			Timestamp:  time.Now(),
			RequestID:  "req-1",
			APIKeyID:   "key-123",
			Provider:   "openai",
			Model:      "gpt-4",
			CostUSD:    0.05,
			ProviderMs: 1000,
			GatewayMs:  1050,
		},
		{
			Timestamp:  time.Now(),
			RequestID:  "req-2",
			APIKeyID:   "key-456",
			Provider:   "anthropic",
			Model:      "claude-3",
			CostUSD:    0.03,
			ProviderMs: 800,
			GatewayMs:  850,
		},
	}

	// Write batch
	key, err := writer.WriteBatch(ctx, records)
	if err != nil {
		t.Fatalf("WriteBatch failed: %v", err)
	}

	if key == "" {
		t.Fatal("Expected non-empty S3 key")
	}

	t.Logf("Wrote batch to S3 key: %s", key)

	// Verify the object exists
	getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(testBucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("Failed to get object from S3: %v", err)
	}
	defer getOutput.Body.Close()

	// Read and verify content
	body, err := io.ReadAll(getOutput.Body)
	if err != nil {
		t.Fatalf("Failed to read object body: %v", err)
	}

	// Parse JSON Lines
	lines := 0
	for _, line := range splitLines(string(body)) {
		if line == "" {
			continue
		}
		var record LogRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("Failed to parse JSON line: %v", err)
		}
		lines++
	}

	if lines != len(records) {
		t.Errorf("Expected %d lines, got %d", len(records), lines)
	}

	// Verify content type
	if getOutput.ContentType == nil || *getOutput.ContentType != "application/x-ndjson" {
		t.Errorf("Expected content type application/x-ndjson, got %v", getOutput.ContentType)
	}
}

// TestS3Integration_S3Sink tests the full S3 sink with enqueue and flush
func TestS3Integration_S3Sink(t *testing.T) {
	if !isMinioAvailable(t) {
		return
	}

	client, err := createMinioClient()
	if err != nil {
		t.Fatalf("Failed to create Minio client: %v", err)
	}

	setupTestBucket(t, client)
	defer cleanupTestBucket(t, client)

	ctx := context.Background()

	// Create sink with small buffer and flush size for testing
	sinkConfig := S3SinkConfig{
		BufferSize:    100,
		FlushSize:     5, // Flush after 5 records
		FlushInterval: 1 * time.Second,
		S3Bucket:      testBucketName,
		S3Region:      "us-east-1",
		S3Prefix:      "sink-test/",
		PodName:       "test-pod-1",
	}

	// Create custom S3 writer with Minio client
	writer := &S3Writer{
		client:  client,
		bucket:  testBucketName,
		prefix:  sinkConfig.S3Prefix,
		podName: sinkConfig.PodName,
		logger:  NewTestLogger(),
	}

	// Create sink manually to inject our Minio-configured writer
	sink := createTestS3Sink(t, sinkConfig, writer)

	// Enqueue records
	for i := 0; i < 10; i++ {
		record := &LogRecord{
			Timestamp:  time.Now(),
			RequestID:  fmt.Sprintf("req-%d", i),
			APIKeyID:   "test-key",
			Provider:   "openai",
			Model:      "gpt-4",
			CostUSD:    0.01 * float64(i+1),
			ProviderMs: int64(100 * (i + 1)),
			GatewayMs:  int64(100*(i+1) + 50),
		}

		if err := sink.Enqueue(record); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Wait for first flush (should trigger at 5 records)
	time.Sleep(1 * time.Second)

	// Force shutdown to flush remaining 5 records
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := sink.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	cancel()

	// Give S3 a moment to process
	time.Sleep(500 * time.Millisecond)

	// List objects in bucket
	listOutput, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(testBucketName),
		Prefix: aws.String(sinkConfig.S3Prefix),
	})
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}

	if len(listOutput.Contents) == 0 {
		t.Fatal("Expected at least one object in S3, got none")
	}

	t.Logf("Found %d objects in S3", len(listOutput.Contents))

	// Verify total records across all files
	totalRecords := 0
	for _, obj := range listOutput.Contents {
		getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucketName),
			Key:    obj.Key,
		})
		if err != nil {
			t.Fatalf("Failed to get object %s: %v", *obj.Key, err)
		}

		body, err := io.ReadAll(getOutput.Body)
		getOutput.Body.Close()
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		lines := splitLines(string(body))
		for _, line := range lines {
			if line != "" {
				totalRecords++
			}
		}
	}

	if totalRecords != 10 {
		t.Errorf("Expected 10 total records in S3, got %d", totalRecords)
	}
}

// TestS3Integration_GracefulShutdown tests that shutdown flushes remaining records
func TestS3Integration_GracefulShutdown(t *testing.T) {
	if !isMinioAvailable(t) {
		return
	}

	client, err := createMinioClient()
	if err != nil {
		t.Fatalf("Failed to create Minio client: %v", err)
	}

	setupTestBucket(t, client)
	defer cleanupTestBucket(t, client)

	ctx := context.Background()

	sinkConfig := S3SinkConfig{
		BufferSize:    100,
		FlushSize:     100, // High flush size so it won't auto-flush
		FlushInterval: 10 * time.Minute,
		S3Bucket:      testBucketName,
		S3Region:      "us-east-1",
		S3Prefix:      "shutdown-test/",
		PodName:       "shutdown-pod",
	}

	writer := &S3Writer{
		client:  client,
		bucket:  testBucketName,
		prefix:  sinkConfig.S3Prefix,
		podName: sinkConfig.PodName,
		logger:  NewTestLogger(),
	}

	sink := createTestS3Sink(t, sinkConfig, writer)

	// Enqueue a few records (less than flush size)
	for i := 0; i < 3; i++ {
		record := &LogRecord{
			Timestamp: time.Now(),
			RequestID: fmt.Sprintf("shutdown-req-%d", i),
			APIKeyID:  "shutdown-key",
			Provider:  "openai",
			Model:     "gpt-4",
			CostUSD:   0.01,
		}
		if err := sink.Enqueue(record); err != nil {
			t.Fatalf("Enqueue failed: %v", err)
		}
	}

	// Shutdown should flush remaining records
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sink.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Verify records were flushed
	time.Sleep(500 * time.Millisecond)

	listOutput, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(testBucketName),
		Prefix: aws.String(sinkConfig.S3Prefix),
	})
	if err != nil {
		t.Fatalf("Failed to list objects: %v", err)
	}

	if len(listOutput.Contents) == 0 {
		t.Fatal("Expected shutdown to flush records to S3, but no objects found")
	}

	// Count records
	totalRecords := 0
	for _, obj := range listOutput.Contents {
		getOutput, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(testBucketName),
			Key:    obj.Key,
		})
		if err != nil {
			t.Fatalf("Failed to get object: %v", err)
		}

		body, err := io.ReadAll(getOutput.Body)
		getOutput.Body.Close()
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}

		lines := splitLines(string(body))
		for _, line := range lines {
			if line != "" {
				totalRecords++
			}
		}
	}

	if totalRecords != 3 {
		t.Errorf("Expected 3 records flushed on shutdown, got %d", totalRecords)
	}
}

// Helper function to split string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Helper to create test S3 sink with custom writer
func createTestS3Sink(t *testing.T, config S3SinkConfig, writer *S3Writer) *S3Sink {
	queueConfig := &queue.Config{
		QueueName:    "test-logging",
		BatchSize:    config.FlushSize,
		BatchTimeout: config.FlushInterval,
		MaxRetries:   0,
		RetryBackoff: 0,
		UseRedis:     false,
	}
	memQueue := queue.NewMemoryQueue(queueConfig)

	sink := &S3Sink{
		queue:         memQueue,
		writer:        writer,
		flushSize:     config.FlushSize,
		flushInterval: config.FlushInterval,
		logger:        NewTestLogger(),
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
	}

	// Start background worker
	sink.wg.Add(1)
	go sink.run(context.Background())

	return sink
}

// NewTestLogger creates a logger for testing
func NewTestLogger() *utils.Logger {
	return utils.NewLogger("test")
}
