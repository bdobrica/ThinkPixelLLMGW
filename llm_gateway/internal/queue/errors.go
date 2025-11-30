package queue

import "errors"

var (
	// ErrQueueClosed is returned when operating on a closed queue
	ErrQueueClosed = errors.New("queue is closed")

	// ErrItemNotFound is returned when an item is not found
	ErrItemNotFound = errors.New("item not found")

	// ErrMaxRetriesExceeded is returned when max retries are exceeded
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)
