package httpapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"llm_gateway/internal/queue"
)

type closeCountingQueue struct {
	mu       sync.Mutex
	closes   int
	closeErr error
}

func (*closeCountingQueue) Enqueue(context.Context, interface{}) error          { return nil }
func (*closeCountingQueue) Dequeue(context.Context, int) ([]interface{}, error) { return nil, nil }
func (*closeCountingQueue) DequeueWithTimeout(context.Context, int, time.Duration) ([]interface{}, error) {
	return nil, nil
}
func (*closeCountingQueue) Length(context.Context) (int, error) { return 0, nil }
func (q *closeCountingQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closes++
	return q.closeErr
}

type closeCountingDLQ struct{ closeCountingQueue }

func (*closeCountingDLQ) Add(context.Context, interface{}, error) error             { return nil }
func (*closeCountingDLQ) List(context.Context, int) ([]queue.DeadLetterItem, error) { return nil, nil }
func (*closeCountingDLQ) Remove(context.Context, string) error                      { return nil }

func TestDependenciesCloseClosesOwnedQueuesExactlyOnceAndAggregatesErrors(t *testing.T) {
	billingQueue := &closeCountingQueue{closeErr: errors.New("billing unavailable")}
	billingDLQ := &closeCountingDLQ{}
	usageQueue := &closeCountingQueue{closeErr: errors.New("usage unavailable")}
	usageDLQ := &closeCountingDLQ{}
	deps := &Dependencies{
		billingQueue: billingQueue,
		billingDLQ:   billingDLQ,
		usageQueue:   usageQueue,
		usageDLQ:     usageDLQ,
	}

	err := deps.Close(context.Background())
	if err == nil || !strings.Contains(err.Error(), "billing unavailable") || !strings.Contains(err.Error(), "usage unavailable") {
		t.Fatalf("Close() error = %v, want both resource errors", err)
	}
	if secondErr := deps.Close(context.Background()); secondErr == nil || secondErr.Error() != err.Error() {
		t.Fatalf("second Close() error = %v, want cached %v", secondErr, err)
	}
	for name, count := range map[string]int{
		"billing queue": billingQueue.closes,
		"billing DLQ":   billingDLQ.closes,
		"usage queue":   usageQueue.closes,
		"usage DLQ":     usageDLQ.closes,
	} {
		if count != 1 {
			t.Errorf("%s closed %d times, want 1", name, count)
		}
	}
}

func TestDependenciesCloseHonorsExpiredWorkerDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deps := &Dependencies{}
	if err := deps.Close(ctx); err != nil {
		t.Fatalf("Close() with no blocking resources = %v", err)
	}
}
