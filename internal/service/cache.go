package service

import (
	"maps"
	"sync"
)

type ConfigCache struct {
	mu       sync.RWMutex
	data     map[string]string
	revision int64 // the version of the current cache
}

func NewConfigCache() *ConfigCache {
	return &ConfigCache{
		data: make(map[string]string),
	}
}

func (c *ConfigCache) Update(key, val string, rev int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = val
	if rev > c.revision {
		c.revision = rev
	}
}

func (c *ConfigCache) Delete(key string, rev int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	if rev > c.revision {
		c.revision = rev
	}
}

func (c *ConfigCache) GetSnapshot() (map[string]string, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make(map[string]string, len(c.data))
	maps.Copy(res, c.data)
	return res, c.revision
}
