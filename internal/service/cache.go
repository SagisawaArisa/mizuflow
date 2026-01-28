package service

import (
	v1 "mizuflow/pkg/api/v1"
	"sync"
)

type FeatureCache struct {
	mu       sync.RWMutex
	data     map[string]v1.FeatureFlag
	revision int64 // the version of the current cache
}

func NewFeatureCache() *FeatureCache {
	return &FeatureCache{
		data: make(map[string]v1.FeatureFlag),
	}
}

func (c *FeatureCache) Update(f v1.FeatureFlag) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[f.Key] = f
	if f.Revision > c.revision {
		c.revision = f.Revision
	}
}

func (c *FeatureCache) Delete(key string, rev int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	if rev > c.revision {
		c.revision = rev
	}
}

func (c *FeatureCache) GetSnapshot() ([]v1.FeatureFlag, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]v1.FeatureFlag, 0, len(c.data))
	for _, f := range c.data {
		res = append(res, f)
	}
	return res, c.revision
}
