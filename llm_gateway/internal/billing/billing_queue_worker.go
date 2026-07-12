package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"llm_gateway/internal/models"
	"llm_gateway/internal/queue"
	"llm_gateway/internal/utils"
)

// BillingUpdate represents a billing update to be processed
type BillingUpdate struct {
	SchemaVersion int            `json:"schema_version"`
	APIKeyID      string         `json:"api_key_id"`
	CostNanoUSD   models.NanoUSD `json:"cost_nano_usd"`
	CostUSD       float64        `json:"cost_usd,omitempty"` // v1 dual-read compatibility
	Timestamp     time.Time      `json:"timestamp"`
}

func (u *BillingUpdate) normalizeCurrency() error {
	if u.CostNanoUSD == 0 && u.CostUSD != 0 {
		value, err := models.NanoUSDFromFloat64(u.CostUSD)
		if err != nil {
			return err
		}
		u.CostNanoUSD = value
	}
	u.CostUSD = u.CostNanoUSD.Float64()
	if u.SchemaVersion == 0 {
		u.SchemaVersion = 2
	}
	return nil
}

// BillingQueueWorker processes billing updates asynchronously
type BillingQueueWorker struct {
	queue       queue.Queue
	dlq         queue.DeadLetterQueue
	service     Service
	config      *queue.Config
	stopChan    chan struct{}
	stoppedChan chan struct{}
	stopOnce    sync.Once
	running     atomic.Bool
}

// NewBillingQueueWorker creates a new billing queue worker
func NewBillingQueueWorker(q queue.Queue, dlq queue.DeadLetterQueue, service Service, config *queue.Config) *BillingQueueWorker {
	if config == nil {
		config = queue.DefaultConfig("billing")
	}

	return &BillingQueueWorker{
		queue:       q,
		dlq:         dlq,
		service:     service,
		config:      config,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}
}

// Start starts the worker goroutine
func (w *BillingQueueWorker) Start(ctx context.Context) {
	w.running.Store(true)
	go w.run(ctx)
}

// Ready reports whether the worker is running and able to accept queued work.
func (w *BillingQueueWorker) Ready() bool { return w.running.Load() }

// Stop gracefully stops the worker
func (w *BillingQueueWorker) Stop() error {
	return w.StopContext(context.Background())
}

// StopContext gracefully stops the worker without waiting beyond ctx's deadline.
func (w *BillingQueueWorker) StopContext(ctx context.Context) error {
	w.stopOnce.Do(func() { close(w.stopChan) })
	select {
	case <-w.stoppedChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Enqueue adds a billing update to the queue
func (w *BillingQueueWorker) Enqueue(ctx context.Context, update *BillingUpdate) error {
	if err := update.normalizeCurrency(); err != nil {
		return err
	}
	return w.queue.Enqueue(ctx, update)
}

// run is the main worker loop
func (w *BillingQueueWorker) run(ctx context.Context) {
	defer close(w.stoppedChan)
	defer w.running.Store(false)

	logger := utils.NewLogger("billing-worker")

	for {
		select {
		case <-w.stopChan:
			logger.Info("Billing worker stopping")
			return
		case <-ctx.Done():
			logger.Info("Billing worker context cancelled")
			return
		default:
			w.processBatch(ctx, logger)
		}
	}
}

// processBatch processes a batch of billing updates
func (w *BillingQueueWorker) processBatch(ctx context.Context, logger *utils.Logger) {
	// Dequeue items with timeout
	items, err := w.queue.DequeueWithTimeout(ctx, w.config.BatchSize, w.config.BatchTimeout)
	if err != nil {
		logger.Error("Failed to dequeue billing updates", "error", err)
		time.Sleep(1 * time.Second) // Back off on error
		return
	}

	if len(items) == 0 {
		return
	}

	logger.Debug("Processing billing batch", "count", len(items))

	// Process each item
	for _, item := range items {
		if err := w.processItem(ctx, item, logger); err != nil {
			logger.Error("Failed to process billing update", "error", err)
		}
	}
}

// processItem processes a single billing update with retries
func (w *BillingQueueWorker) processItem(ctx context.Context, item interface{}, logger *utils.Logger) error {
	// Unmarshal item
	var update BillingUpdate
	if err := w.unmarshalItem(item, &update); err != nil {
		logger.Error("Failed to unmarshal billing update", "error", err)
		return err
	}
	if err := update.normalizeCurrency(); err != nil {
		return err
	}

	// Try to process with retries
	var lastErr error
	for attempt := 0; attempt <= w.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := w.config.RetryBackoff * time.Duration(1<<uint(attempt-1))
			logger.Debug("Retrying billing update", "attempt", attempt, "backoff", backoff)
			time.Sleep(backoff)
		}

		// Process the update
		if err := w.service.AddUsage(ctx, update.APIKeyID, update.CostUSD); err != nil {
			lastErr = err
			logger.Error("Failed to add usage", "attempt", attempt, "error", err)
			continue
		}

		// Success
		logger.Debug("Billing update processed", "api_key_id", update.APIKeyID, "cost", update.CostUSD)
		return nil
	}

	// Max retries exceeded - add to dead letter queue
	if w.dlq != nil {
		if err := w.dlq.Add(ctx, update, lastErr); err != nil {
			logger.Error("Failed to add to dead letter queue", "error", err)
		} else {
			logger.Warn("Billing update moved to DLQ", "api_key_id", update.APIKeyID, "error", lastErr)
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// unmarshalItem unmarshals a queue item into a BillingUpdate
func (w *BillingQueueWorker) unmarshalItem(item interface{}, update *BillingUpdate) error {
	switch v := item.(type) {
	case *BillingUpdate:
		*update = *v
		return nil
	case BillingUpdate:
		*update = v
		return nil
	case []byte:
		return json.Unmarshal(v, update)
	case json.RawMessage:
		return json.Unmarshal(v, update)
	default:
		// Try to marshal and unmarshal
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}
		return json.Unmarshal(data, update)
	}
}

// GetQueueLength returns the current queue length
func (w *BillingQueueWorker) GetQueueLength(ctx context.Context) (int, error) {
	return w.queue.Length(ctx)
}

// GetDeadLetterItems returns items from the dead letter queue
func (w *BillingQueueWorker) GetDeadLetterItems(ctx context.Context, maxItems int) ([]queue.DeadLetterItem, error) {
	if w.dlq == nil {
		return nil, fmt.Errorf("dead letter queue not configured")
	}
	return w.dlq.List(ctx, maxItems)
}

// RetryDeadLetterItem retries a failed item from the dead letter queue
func (w *BillingQueueWorker) RetryDeadLetterItem(ctx context.Context, id string) error {
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
