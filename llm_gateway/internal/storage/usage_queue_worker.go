package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"llm_gateway/internal/models"
	"llm_gateway/internal/queue"
	"llm_gateway/internal/utils"
)

// UsageQueueWorker processes usage records asynchronously
type UsageQueueWorker struct {
	queue       queue.Queue
	dlq         queue.DeadLetterQueue
	db          *DB
	config      *queue.Config
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// NewUsageQueueWorker creates a new usage queue worker
func NewUsageQueueWorker(q queue.Queue, dlq queue.DeadLetterQueue, db *DB, config *queue.Config) *UsageQueueWorker {
	if config == nil {
		config = queue.DefaultConfig("usage")
	}

	return &UsageQueueWorker{
		queue:       q,
		dlq:         dlq,
		db:          db,
		config:      config,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}
}

// Start starts the worker goroutine
func (w *UsageQueueWorker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop gracefully stops the worker
func (w *UsageQueueWorker) Stop() error {
	close(w.stopChan)
	<-w.stoppedChan
	return nil
}

// Enqueue adds a usage record to the queue
func (w *UsageQueueWorker) Enqueue(ctx context.Context, record *models.UsageRecord) error {
	return w.queue.Enqueue(ctx, record)
}

// run is the main worker loop
func (w *UsageQueueWorker) run(ctx context.Context) {
	defer close(w.stoppedChan)

	logger := utils.NewLogger("usage-worker")

	for {
		select {
		case <-w.stopChan:
			logger.Info("Usage worker stopping")
			return
		case <-ctx.Done():
			logger.Info("Usage worker context cancelled")
			return
		default:
			w.processBatch(ctx, logger)
		}
	}
}

// processBatch processes a batch of usage records
func (w *UsageQueueWorker) processBatch(ctx context.Context, logger *utils.Logger) {
	// Dequeue items with timeout
	items, err := w.queue.DequeueWithTimeout(ctx, w.config.BatchSize, w.config.BatchTimeout)
	if err != nil {
		logger.Error("Failed to dequeue usage records", "error", err)
		time.Sleep(1 * time.Second) // Back off on error
		return
	}

	if len(items) == 0 {
		return
	}

	logger.Debug("Processing usage batch", "count", len(items))

	// Convert to usage records
	records := make([]*models.UsageRecord, 0, len(items))
	for _, item := range items {
		var record models.UsageRecord
		if err := w.unmarshalItem(item, &record); err != nil {
			logger.Error("Failed to unmarshal usage record", "error", err)
			continue
		}
		records = append(records, &record)
	}

	if len(records) == 0 {
		return
	}

	// Try to insert batch
	if err := w.insertBatch(ctx, records, logger); err != nil {
		logger.Error("Failed to insert batch, falling back to individual inserts", "error", err)
		// Fall back to individual inserts with retries
		for _, record := range records {
			if err := w.processItem(ctx, record, logger); err != nil {
				logger.Error("Failed to process usage record", "error", err)
			}
		}
	}
}

// insertBatch inserts multiple usage records in a single transaction
func (w *UsageQueueWorker) insertBatch(ctx context.Context, records []*models.UsageRecord, logger *utils.Logger) error {
	repo := NewUsageRepository(w.db)

	// Start transaction
	tx, err := w.db.conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert each record in the transaction
	for _, record := range records {
		if err := repo.Create(ctx, record); err != nil {
			return fmt.Errorf("failed to insert record: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Debug("Inserted batch successfully", "count", len(records))
	return nil
}

// processItem processes a single usage record with retries
func (w *UsageQueueWorker) processItem(ctx context.Context, record *models.UsageRecord, logger *utils.Logger) error {
	repo := NewUsageRepository(w.db)

	// Try to process with retries
	var lastErr error
	for attempt := 0; attempt <= w.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := w.config.RetryBackoff * time.Duration(1<<uint(attempt-1))
			logger.Debug("Retrying usage record", "attempt", attempt, "backoff", backoff)
			time.Sleep(backoff)
		}

		// Insert the record
		if err := repo.Create(ctx, record); err != nil {
			lastErr = err
			logger.Error("Failed to insert usage record", "attempt", attempt, "error", err)
			continue
		}

		// Success
		logger.Debug("Usage record inserted", "request_id", record.RequestID)
		return nil
	}

	// Max retries exceeded - add to dead letter queue
	if w.dlq != nil {
		if err := w.dlq.Add(ctx, record, lastErr); err != nil {
			logger.Error("Failed to add to dead letter queue", "error", err)
		} else {
			logger.Warn("Usage record moved to DLQ", "request_id", record.RequestID, "error", lastErr)
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// unmarshalItem unmarshals a queue item into a UsageRecord
func (w *UsageQueueWorker) unmarshalItem(item interface{}, record *models.UsageRecord) error {
	switch v := item.(type) {
	case *models.UsageRecord:
		*record = *v
		return nil
	case models.UsageRecord:
		*record = v
		return nil
	case []byte:
		return json.Unmarshal(v, record)
	case json.RawMessage:
		return json.Unmarshal(v, record)
	default:
		// Try to marshal and unmarshal
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}
		return json.Unmarshal(data, record)
	}
}

// GetQueueLength returns the current queue length
func (w *UsageQueueWorker) GetQueueLength(ctx context.Context) (int, error) {
	return w.queue.Length(ctx)
}

// GetDeadLetterItems returns items from the dead letter queue
func (w *UsageQueueWorker) GetDeadLetterItems(ctx context.Context, maxItems int) ([]queue.DeadLetterItem, error) {
	if w.dlq == nil {
		return nil, fmt.Errorf("dead letter queue not configured")
	}
	return w.dlq.List(ctx, maxItems)
}

// RetryDeadLetterItem retries a failed item from the dead letter queue
func (w *UsageQueueWorker) RetryDeadLetterItem(ctx context.Context, id string) error {
	if w.dlq == nil {
		return fmt.Errorf("dead letter queue not configured")
	}

	// Get all items and find the one to retry
	items, err := w.dlq.List(ctx, 0)
	if err != nil {
		return fmt.Errorf("failed to list dead letter items: %w", err)
	}

	for _, dlItem := range items {
		if dlItem.ID == id {
			// Re-enqueue the item
			if err := w.queue.Enqueue(ctx, dlItem.Item); err != nil {
				return fmt.Errorf("failed to re-enqueue item: %w", err)
			}

			// Remove from DLQ
			if err := w.dlq.Remove(ctx, id); err != nil {
				return fmt.Errorf("failed to remove from DLQ: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("item not found in dead letter queue")
}
