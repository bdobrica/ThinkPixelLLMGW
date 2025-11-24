package storage

import (
	"container/list"
	"sync"
	"time"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Key       string
	Value     interface{}
	ExpiresAt time.Time
}

// LRUCache is a thread-safe LRU cache with TTL support
type LRUCache struct {
	mu           sync.RWMutex
	capacity     int
	ttl          time.Duration
	items        map[string]*list.Element
	evictionList *list.List
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		capacity:     capacity,
		ttl:          ttl,
		items:        make(map[string]*list.Element, capacity),
		evictionList: list.New(),
	}
}

// Get retrieves an item from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, found := c.items[key]; found {
		entry := elem.Value.(*CacheEntry)

		// Check if expired
		if time.Now().After(entry.ExpiresAt) {
			c.removeElement(elem)
			return nil, false
		}

		// Move to front (most recently used)
		c.evictionList.MoveToFront(elem)
		return entry.Value, true
	}

	return nil, false
}

// Set adds or updates an item in the cache
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(c.ttl)

	// Update existing item
	if elem, found := c.items[key]; found {
		c.evictionList.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		entry.Value = value
		entry.ExpiresAt = expiresAt
		return
	}

	// Add new item
	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		ExpiresAt: expiresAt,
	}

	elem := c.evictionList.PushFront(entry)
	c.items[key] = elem

	// Evict oldest if over capacity
	if c.evictionList.Len() > c.capacity {
		c.removeOldest()
	}
}

// Delete removes an item from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, found := c.items[key]; found {
		c.removeElement(elem)
	}
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element, c.capacity)
	c.evictionList.Init()
}

// Len returns the current number of items in the cache
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.evictionList.Len()
}

// removeOldest removes the oldest item from the cache
func (c *LRUCache) removeOldest() {
	elem := c.evictionList.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes a specific element from the cache
func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictionList.Remove(elem)
	entry := elem.Value.(*CacheEntry)
	delete(c.items, entry.Key)
}

// CleanupExpired removes all expired items (should be called periodically)
func (c *LRUCache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	// Iterate from back (oldest) to front
	var next *list.Element
	for elem := c.evictionList.Back(); elem != nil; elem = next {
		next = elem.Prev()
		entry := elem.Value.(*CacheEntry)

		if now.After(entry.ExpiresAt) {
			c.removeElement(elem)
			removed++
		}
	}

	return removed
}

// Stats returns cache statistics
type CacheStats struct {
	Capacity int
	Size     int
	TTL      time.Duration
}

// GetStats returns current cache statistics
func (c *LRUCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Capacity: c.capacity,
		Size:     c.evictionList.Len(),
		TTL:      c.ttl,
	}
}
