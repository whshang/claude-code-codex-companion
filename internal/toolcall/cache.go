package toolcall

import (
	"container/list"
	"sync"
	"time"
)

// CacheConfig defines cache configuration
type CacheConfig struct {
	MaxSize int           // Maximum number of entries
	TTL     time.Duration // Time-to-live for entries
}

// cacheEntry represents a single cache entry
type cacheEntry struct {
	key       string
	value     ToolCallMapping
	expiresAt time.Time
	element   *list.Element
}

// ToolCallCache implements LRU cache with TTL for tool call mappings
type ToolCallCache struct {
	config  CacheConfig
	mu      sync.RWMutex
	cache   map[string]*cacheEntry
	lruList *list.List
}

// NewToolCallCache creates a new tool call cache
func NewToolCallCache(config CacheConfig) *ToolCallCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 100 // Default size
	}
	if config.TTL <= 0 {
		config.TTL = 30 * time.Minute // Default TTL
	}

	return &ToolCallCache{
		config:  config,
		cache:   make(map[string]*cacheEntry),
		lruList: list.New(),
	}
}

// Set stores a tool call mapping in the cache
func (c *ToolCallCache) Set(key string, mapping ToolCallMapping) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiresAt := now.Add(c.config.TTL)

	// Check if key already exists
	if entry, exists := c.cache[key]; exists {
		// Update existing entry
		entry.value = mapping
		entry.expiresAt = expiresAt
		// Move to front of LRU list
		c.lruList.MoveToFront(entry.element)
		return
	}

	// Create new entry
	entry := &cacheEntry{
		key:       key,
		value:     mapping,
		expiresAt: expiresAt,
	}

	// Add to front of LRU list
	entry.element = c.lruList.PushFront(entry)
	c.cache[key] = entry

	// Evict oldest entry if cache is full
	if c.lruList.Len() > c.config.MaxSize {
		c.evictOldest()
	}
}

// Get retrieves a tool call mapping from the cache
func (c *ToolCallCache) Get(key string) (ToolCallMapping, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		return ToolCallMapping{}, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		c.removeEntry(entry)
		return ToolCallMapping{}, false
	}

	// Move to front of LRU list (mark as recently used)
	c.lruList.MoveToFront(entry.element)

	return entry.value, true
}

// GetRecent retrieves the N most recent tool call mappings
func (c *ToolCallCache) GetRecent(n int) []ToolCallMapping {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ToolCallMapping, 0, n)
	now := time.Now()

	for elem := c.lruList.Front(); elem != nil && len(result) < n; elem = elem.Next() {
		entry := elem.Value.(*cacheEntry)

		// Skip expired entries
		if now.After(entry.expiresAt) {
			continue
		}

		result = append(result, entry.value)
	}

	return result
}

// CleanExpired removes all expired entries
func (c *ToolCallCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0

	// Iterate from back (oldest) to front
	for elem := c.lruList.Back(); elem != nil; {
		entry := elem.Value.(*cacheEntry)
		prev := elem.Prev()

		if now.After(entry.expiresAt) {
			c.removeEntry(entry)
			removed++
		}

		elem = prev
	}

	return removed
}

// Clear removes all entries from the cache
func (c *ToolCallCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheEntry)
	c.lruList = list.New()
}

// Size returns the current number of entries in the cache
func (c *ToolCallCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lruList.Len()
}

// evictOldest removes the least recently used entry
func (c *ToolCallCache) evictOldest() {
	elem := c.lruList.Back()
	if elem != nil {
		entry := elem.Value.(*cacheEntry)
		c.removeEntry(entry)
	}
}

// removeEntry removes an entry from both cache map and LRU list
func (c *ToolCallCache) removeEntry(entry *cacheEntry) {
	c.lruList.Remove(entry.element)
	delete(c.cache, entry.key)
}

// StartCleanupRoutine starts a background goroutine to periodically clean expired entries
func (c *ToolCallCache) StartCleanupRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.CleanExpired()
		}
	}()
}
