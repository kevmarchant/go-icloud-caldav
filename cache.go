package caldav

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type CacheEntry struct {
	Data       interface{}
	Expiry     time.Time
	LastAccess time.Time
	HitCount   int64
}

type CacheStats struct {
	Hits         int64
	Misses       int64
	Evictions    int64
	TotalEntries int64
	HitRate      float64
}

type ResponseCache struct {
	entries    map[string]*CacheEntry
	mutex      sync.RWMutex
	defaultTTL time.Duration
	maxEntries int
	stats      CacheStats
}

func NewResponseCache(defaultTTL time.Duration, maxEntries int) *ResponseCache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}

	cache := &ResponseCache{
		entries:    make(map[string]*CacheEntry),
		defaultTTL: defaultTTL,
		maxEntries: maxEntries,
	}

	go cache.startCleanupRoutine()
	return cache
}

func (rc *ResponseCache) generateKey(operation, path string, data []byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(operation))
	hasher.Write([]byte(path))
	if data != nil {
		hasher.Write(data)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func (rc *ResponseCache) Get(ctx context.Context, key string) (interface{}, bool) {
	rc.mutex.RLock()
	entry, exists := rc.entries[key]
	rc.mutex.RUnlock()

	if !exists {
		rc.incrementMisses()
		return nil, false
	}

	if time.Now().After(entry.Expiry) {
		rc.mutex.Lock()
		delete(rc.entries, key)
		rc.stats.Evictions++
		rc.mutex.Unlock()
		rc.incrementMisses()
		return nil, false
	}

	rc.mutex.Lock()
	entry.LastAccess = time.Now()
	entry.HitCount++
	rc.mutex.Unlock()

	rc.incrementHits()
	return entry.Data, true
}

func (rc *ResponseCache) Set(key string, data interface{}, ttl time.Duration) {
	if ttl <= 0 {
		ttl = rc.defaultTTL
	}

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if len(rc.entries) >= rc.maxEntries {
		rc.evictLRU()
	}

	rc.entries[key] = &CacheEntry{
		Data:       data,
		Expiry:     time.Now().Add(ttl),
		LastAccess: time.Now(),
		HitCount:   0,
	}
	rc.stats.TotalEntries = int64(len(rc.entries))
}

func (rc *ResponseCache) InvalidatePattern(pattern string) int {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	count := 0
	for key := range rc.entries {
		if matchesPattern(key, pattern) {
			delete(rc.entries, key)
			count++
		}
	}

	rc.stats.Evictions += int64(count)
	rc.stats.TotalEntries = int64(len(rc.entries))
	return count
}

func (rc *ResponseCache) Clear() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.stats.Evictions += int64(len(rc.entries))
	rc.entries = make(map[string]*CacheEntry)
	rc.stats.TotalEntries = 0
}

func (rc *ResponseCache) GetStats() CacheStats {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	stats := rc.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	stats.TotalEntries = int64(len(rc.entries))

	return stats
}

func (rc *ResponseCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range rc.entries {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(rc.entries, oldestKey)
		rc.stats.Evictions++
	}
}

func (rc *ResponseCache) incrementHits() {
	rc.mutex.Lock()
	rc.stats.Hits++
	rc.mutex.Unlock()
}

func (rc *ResponseCache) incrementMisses() {
	rc.mutex.Lock()
	rc.stats.Misses++
	rc.mutex.Unlock()
}

func (rc *ResponseCache) startCleanupRoutine() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rc.cleanupExpired()
	}
}

func (rc *ResponseCache) cleanupExpired() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	now := time.Now()
	for key, entry := range rc.entries {
		if now.After(entry.Expiry) {
			delete(rc.entries, key)
			rc.stats.Evictions++
		}
	}
	rc.stats.TotalEntries = int64(len(rc.entries))
}

func matchesPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) == 0 {
		return false
	}
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return key == pattern
}

type CachedOperation struct {
	Operation string
	Path      string
	Body      []byte
	TTL       time.Duration
}

func (c *CalDAVClient) WithCache(cache *ResponseCache) *CalDAVClient {
	c.cache = cache
	return c
}

func (c *CalDAVClient) getCachedResponse(ctx context.Context, op *CachedOperation) (interface{}, bool) {
	if c.cache == nil {
		return nil, false
	}

	key := c.cache.generateKey(op.Operation, op.Path, op.Body)
	return c.cache.Get(ctx, key)
}

func (c *CalDAVClient) setCachedResponse(op *CachedOperation, data interface{}) {
	if c.cache == nil {
		return
	}

	key := c.cache.generateKey(op.Operation, op.Path, op.Body)
	c.cache.Set(key, data, op.TTL)
}
