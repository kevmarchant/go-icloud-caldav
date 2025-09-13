package caldav

import (
	"context"
	"testing"
	"time"
)

func TestResponseCache_Basic(t *testing.T) {
	cache := NewResponseCache(time.Minute, 10)
	defer cache.Clear()

	key := "test-key"
	value := "test-value"

	_, found := cache.Get(context.Background(), key)
	if found {
		t.Error("expected key not to be found in empty cache")
	}

	cache.Set(key, value, time.Minute)

	result, found := cache.Get(context.Background(), key)
	if !found {
		t.Error("expected key to be found after setting")
	}

	if result != value {
		t.Errorf("expected %v, got %v", value, result)
	}
}

func TestResponseCache_Expiry(t *testing.T) {
	cache := NewResponseCache(10*time.Millisecond, 10)
	defer cache.Clear()

	key := "expiry-test"
	value := "will-expire"

	cache.Set(key, value, 50*time.Millisecond)

	result, found := cache.Get(context.Background(), key)
	if !found {
		t.Error("expected key to be found before expiry")
	}
	if result != value {
		t.Errorf("expected %v, got %v", value, result)
	}

	time.Sleep(100 * time.Millisecond)

	_, found = cache.Get(context.Background(), key)
	if found {
		t.Error("expected key to be expired and not found")
	}
}

func TestResponseCache_MaxEntries(t *testing.T) {
	cache := NewResponseCache(time.Minute, 3)
	defer cache.Clear()

	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		value := i
		cache.Set(key, value, time.Minute)
	}

	stats := cache.GetStats()
	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 entries, got %d", stats.TotalEntries)
	}
}

func TestResponseCache_Stats(t *testing.T) {
	cache := NewResponseCache(time.Minute, 10)
	defer cache.Clear()

	key := "stats-test"
	value := "test-value"

	_, found := cache.Get(context.Background(), key)
	if found {
		t.Error("unexpected cache hit on empty cache")
	}

	cache.Set(key, value, time.Minute)

	_, found = cache.Get(context.Background(), key)
	if !found {
		t.Error("expected cache hit")
	}

	_, found = cache.Get(context.Background(), key)
	if !found {
		t.Error("expected second cache hit")
	}

	stats := cache.GetStats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.HitRate != 2.0/3.0 {
		t.Errorf("expected hit rate %.2f, got %.2f", 2.0/3.0, stats.HitRate)
	}
}

func TestResponseCache_InvalidatePattern(t *testing.T) {
	cache := NewResponseCache(time.Minute, 10)
	defer cache.Clear()

	cache.Set("user:123:profile", "profile-data", time.Minute)
	cache.Set("user:123:calendar", "calendar-data", time.Minute)
	cache.Set("user:456:profile", "other-profile", time.Minute)

	invalidated := cache.InvalidatePattern("user:123:*")
	if invalidated != 2 {
		t.Errorf("expected 2 invalidated entries, got %d", invalidated)
	}

	_, found := cache.Get(context.Background(), "user:123:profile")
	if found {
		t.Error("expected user:123:profile to be invalidated")
	}

	_, found = cache.Get(context.Background(), "user:123:calendar")
	if found {
		t.Error("expected user:123:calendar to be invalidated")
	}

	_, found = cache.Get(context.Background(), "user:456:profile")
	if !found {
		t.Error("expected user:456:profile to remain")
	}
}

func TestCacheKey_Generation(t *testing.T) {
	cache := NewResponseCache(time.Minute, 10)
	defer cache.Clear()

	key1 := cache.generateKey("op1", "/path", []byte("data"))
	key2 := cache.generateKey("op1", "/path", []byte("data"))
	key3 := cache.generateKey("op1", "/path", []byte("different-data"))

	if key1 != key2 {
		t.Error("identical inputs should generate same key")
	}
	if key1 == key3 {
		t.Error("different data should generate different key")
	}
}

func TestClientOption_WithCache(t *testing.T) {
	client := NewClientWithOptions("user", "pass", WithCache(time.Minute, 100))

	if client.cache == nil {
		t.Error("expected cache to be set on client")
	}

	stats := client.cache.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("expected empty cache, got %d entries", stats.TotalEntries)
	}
}

func TestClientOption_WithExistingCache(t *testing.T) {
	existingCache := NewResponseCache(time.Hour, 500)
	existingCache.Set("existing-key", "existing-value", time.Hour)

	client := NewClientWithOptions("user", "pass", WithExistingCache(existingCache))

	if client.cache != existingCache {
		t.Error("expected client to use existing cache instance")
	}

	cached, found := client.cache.Get(context.Background(), "existing-key")
	if !found {
		t.Error("expected to find existing cache entry")
	}
	if cached != "existing-value" {
		t.Errorf("expected 'existing-value', got %v", cached)
	}
}

func TestCalDAVClient_CacheIntegration(t *testing.T) {
	client := NewClientWithOptions("user", "pass", WithCache(time.Minute, 100))

	cacheOp := &CachedOperation{
		Operation: "test-op",
		Path:      "/test/path",
		Body:      []byte("test-body"),
		TTL:       time.Minute,
	}

	_, found := client.getCachedResponse(context.Background(), cacheOp)
	if found {
		t.Error("expected no cached response initially")
	}

	testData := "cached-response-data"
	client.setCachedResponse(cacheOp, testData)

	cached, found := client.getCachedResponse(context.Background(), cacheOp)
	if !found {
		t.Error("expected to find cached response after setting")
	}
	if cached != testData {
		t.Errorf("expected %v, got %v", testData, cached)
	}
}
