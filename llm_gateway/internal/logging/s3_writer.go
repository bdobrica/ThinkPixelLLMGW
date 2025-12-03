package logging

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"llm_gateway/internal/utils"
)

// S3Writer writes log records to S3 in JSON Lines format
type S3Writer struct {
	client  *s3.Client
	bucket  string
	prefix  string
	podName string
	logger  *utils.Logger
}

// NewS3Writer creates a new S3 writer
func NewS3Writer(ctx context.Context, bucket, region, prefix, podName string) (*S3Writer, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with path-style addressing for Minio compatibility
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Writer{
		client:  client,
		bucket:  bucket,
		prefix:  prefix,
		podName: podName,
		logger:  utils.NewLogger("s3-writer", utils.Info),
	}, nil
}

// WriteBatch writes a batch of log records to S3 as a gzip-compressed JSON Lines file
// Returns the S3 key where the batch was written
func (w *S3Writer) WriteBatch(ctx context.Context, records []*LogRecord) (string, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("no records to write")
	}

	// Generate S3 key based on timestamp
	// Format: logs/<year>/<month>/<day>/<pod>-<timestamp>-<nano>.jsonl.gz
	now := time.Now().UTC()
	key := fmt.Sprintf("%s%04d/%02d/%02d/%s-%d-%d.jsonl.gz",
		w.prefix,
		now.Year(),
		now.Month(),
		now.Day(),
		w.podName,
		now.Unix(),
		now.Nanosecond(),
	)

	// Serialize records to JSON Lines format with gzip compression
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	encoder := json.NewEncoder(gzipWriter)

	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			gzipWriter.Close()
			return "", fmt.Errorf("failed to encode record: %w", err)
		}
	}

	// Close gzip writer to flush remaining data
	if err := gzipWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Upload to S3
	_, err := w.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:          aws.String(w.bucket),
		Key:             aws.String(key),
		Body:            bytes.NewReader(buf.Bytes()),
		ContentType:     aws.String("application/x-ndjson"),
		ContentEncoding: aws.String("gzip"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	w.logger.Debug("Wrote batch to S3",
		"key", key,
		"records", len(records),
		"bytes", buf.Len(),
	)

	return key, nil
}
