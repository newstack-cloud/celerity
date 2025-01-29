package core

import "sync"

// Cache provides an implementation of a cache
// that can be used across go routines.
// This is a thread safe cache to be used by internal
// components of the blueprint framework,
// not to be confused with the `cache.BlueprintCache` interface
// which is utility interface on which applications can build
// their own blueprint caching implementations.
type Cache[Data any] struct {
	data map[string]Data
	mu   sync.RWMutex
}

// NewCache provides a thread safe cache that can be used
// across go routines.
func NewCache[Data any]() *Cache[Data] {
	return &Cache[Data]{
		data: make(map[string]Data),
	}
}

// Get an item from the cache.
func (c *Cache[Data]) Get(key string) (Data, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.data[key]
	return data, ok
}

// Set an item in the cache.
func (c *Cache[Data]) Set(key string, data Data) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = data
}

// Delete an item from the cache.
func (c *Cache[Data]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}
