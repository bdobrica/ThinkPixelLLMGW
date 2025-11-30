package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"llm_gateway/internal/utils"
)

// S3Writer handles writing batches of log records to S3
type S3Writer struct {
	client  *s3.Client
	bucket  string
	prefix  string
	podName string
	logger  *utils.Logger
}

// NewS3Writer creates a new S3 writer
func NewS3Writer(ctx context.Context, bucket, region, prefix, podName string) (*S3Writer, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &S3Writer{
		client:  client,
		bucket:  bucket,
		prefix:  prefix,
		podName: podName,
		logger:  utils.NewLogger("s3-writer"),
	}, nil
}

// WriteBatch writes a batch of log records to S3 as a JSON Lines file
// Returns the S3 key where the data was written
func (w *S3Writer) WriteBatch(ctx context.Context, records []*LogRecord) (string, error) {
	if len(records) == 0 {
		return "", nil
	}

	// Generate S3 key with timestamp and pod name
	// Format: logs/2025/11/30/gateway-0-20251130-143022-123456789.jsonl
	now := time.Now()
	key := fmt.Sprintf("%s%04d/%02d/%02d/%s-%s-%d.jsonl",
		w.prefix,
		now.Year(),
		now.Month(),
		now.Day(),
		w.podName,
		now.Format("20060102-150405"),
		now.Nanosecond(),
	)

	// Convert records to JSON Lines format
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			w.logger.Error("Failed to encode record", "error", err)
			continue
		}
	}

	// Upload to S3
	_, err := w.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(w.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/x-ndjson"),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	w.logger.Info("Wrote batch to S3", "key", key, "count", len(records), "bytes", buf.Len())
	return key, nil
}
