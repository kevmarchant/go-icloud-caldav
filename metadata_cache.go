package caldav

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type MetadataType int

const (
	MetadataCalendar MetadataType = iota
	MetadataPrincipal
	MetadataHomeSet
	MetadataCalendarList
	MetadataPrivileges
	MetadataQuota
)

type MetadataCacheEntry struct {
	Type         MetadataType
	Data         interface{}
	ETag         string
	LastModified time.Time
	Expiry       time.Time
	LastAccess   time.Time
	HitCount     int64
	Size         int64
}

type MetadataCacheConfig struct {
	CalendarTTL       time.Duration
	PrincipalTTL      time.Duration
	HomeSetTTL        time.Duration
	CalendarListTTL   time.Duration
	PrivilegesTTL     time.Duration
	QuotaTTL          time.Duration
	MaxEntries        int
	MaxMemoryMB       int64
	EnableAutoRefresh bool
	RefreshInterval   time.Duration
}

func DefaultMetadataCacheConfig() *MetadataCacheConfig {
	return &MetadataCacheConfig{
		CalendarTTL:       15 * time.Minute,
		PrincipalTTL:      30 * time.Minute,
		HomeSetTTL:        30 * time.Minute,
		CalendarListTTL:   10 * time.Minute,
		PrivilegesTTL:     60 * time.Minute,
		QuotaTTL:          5 * time.Minute,
		MaxEntries:        1000,
		MaxMemoryMB:       100,
		EnableAutoRefresh: false,
		RefreshInterval:   5 * time.Minute,
	}
}

type MetadataCache struct {
	entries          map[string]*MetadataCacheEntry
	mu               sync.RWMutex
	config           *MetadataCacheConfig
	stats            *MetadataCacheStats
	refreshCallbacks map[MetadataType]func(ctx context.Context, key string) (interface{}, error)
	stopChan         chan struct{}
	client           *CalDAVClient
}

type MetadataCacheStats struct {
	TotalHits        int64
	TotalMisses      int64
	CalendarHits     int64
	CalendarMisses   int64
	PrincipalHits    int64
	PrincipalMisses  int64
	HomeSetHits      int64
	HomeSetMisses    int64
	ListHits         int64
	ListMisses       int64
	Evictions        int64
	RefreshCount     int64
	TotalEntries     int64
	TotalMemoryBytes int64
}

func NewMetadataCache(config *MetadataCacheConfig) *MetadataCache {
	if config == nil {
		config = DefaultMetadataCacheConfig()
	}

	cache := &MetadataCache{
		entries:          make(map[string]*MetadataCacheEntry),
		config:           config,
		stats:            &MetadataCacheStats{},
		refreshCallbacks: make(map[MetadataType]func(ctx context.Context, key string) (interface{}, error)),
		stopChan:         make(chan struct{}),
	}

	if config.EnableAutoRefresh {
		go cache.startMaintenanceRoutine()
	}

	return cache
}

func (mc *MetadataCache) Get(ctx context.Context, metaType MetadataType, key string) (interface{}, bool) {
	mc.mu.RLock()
	entry, exists := mc.entries[mc.buildKey(metaType, key)]
	mc.mu.RUnlock()

	if !exists {
		mc.recordMiss(metaType)
		return nil, false
	}

	if time.Now().After(entry.Expiry) {
		mc.mu.Lock()
		delete(mc.entries, mc.buildKey(metaType, key))
		atomic.AddInt64(&mc.stats.Evictions, 1)
		mc.mu.Unlock()

		mc.recordMiss(metaType)

		if mc.config.EnableAutoRefresh {
			go mc.refreshEntry(ctx, metaType, key)
		}

		return nil, false
	}

	mc.mu.Lock()
	entry.LastAccess = time.Now()
	entry.HitCount++
	mc.mu.Unlock()

	mc.recordHit(metaType)
	return entry.Data, true
}

func (mc *MetadataCache) Set(metaType MetadataType, key string, data interface{}, etag string) {
	ttl := mc.getTTLForType(metaType)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if len(mc.entries) >= mc.config.MaxEntries {
		mc.evictLRU()
	}

	size := mc.estimateSize(data)
	if mc.wouldExceedMemoryLimit(size) {
		mc.evictUntilMemoryAvailable(size)
	}

	fullKey := mc.buildKey(metaType, key)
	mc.entries[fullKey] = &MetadataCacheEntry{
		Type:         metaType,
		Data:         data,
		ETag:         etag,
		LastModified: time.Now(),
		Expiry:       time.Now().Add(ttl),
		LastAccess:   time.Now(),
		HitCount:     0,
		Size:         size,
	}

	atomic.StoreInt64(&mc.stats.TotalEntries, int64(len(mc.entries)))
	mc.updateMemoryStats()
}

func (mc *MetadataCache) InvalidateType(metaType MetadataType) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	count := 0
	for key, entry := range mc.entries {
		if entry.Type == metaType {
			delete(mc.entries, key)
			count++
		}
	}

	atomic.AddInt64(&mc.stats.Evictions, int64(count))
	atomic.StoreInt64(&mc.stats.TotalEntries, int64(len(mc.entries)))
	mc.updateMemoryStats()

	return count
}

func (mc *MetadataCache) InvalidateKey(metaType MetadataType, key string) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	fullKey := mc.buildKey(metaType, key)
	if _, exists := mc.entries[fullKey]; exists {
		delete(mc.entries, fullKey)
		atomic.AddInt64(&mc.stats.Evictions, 1)
		atomic.StoreInt64(&mc.stats.TotalEntries, int64(len(mc.entries)))
		mc.updateMemoryStats()
		return true
	}
	return false
}

func (mc *MetadataCache) SetRefreshCallback(metaType MetadataType, callback func(ctx context.Context, key string) (interface{}, error)) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.refreshCallbacks[metaType] = callback
}

func (mc *MetadataCache) GetStats() MetadataCacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := *mc.stats
	stats.TotalEntries = int64(len(mc.entries))
	return stats
}

func (mc *MetadataCache) GetHitRate(metaType MetadataType) float64 {
	var hits, misses int64

	switch metaType {
	case MetadataCalendar:
		hits = atomic.LoadInt64(&mc.stats.CalendarHits)
		misses = atomic.LoadInt64(&mc.stats.CalendarMisses)
	case MetadataPrincipal:
		hits = atomic.LoadInt64(&mc.stats.PrincipalHits)
		misses = atomic.LoadInt64(&mc.stats.PrincipalMisses)
	case MetadataHomeSet:
		hits = atomic.LoadInt64(&mc.stats.HomeSetHits)
		misses = atomic.LoadInt64(&mc.stats.HomeSetMisses)
	case MetadataCalendarList:
		hits = atomic.LoadInt64(&mc.stats.ListHits)
		misses = atomic.LoadInt64(&mc.stats.ListMisses)
	default:
		hits = atomic.LoadInt64(&mc.stats.TotalHits)
		misses = atomic.LoadInt64(&mc.stats.TotalMisses)
	}

	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

func (mc *MetadataCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	count := len(mc.entries)
	mc.entries = make(map[string]*MetadataCacheEntry)
	atomic.AddInt64(&mc.stats.Evictions, int64(count))
	atomic.StoreInt64(&mc.stats.TotalEntries, 0)
	atomic.StoreInt64(&mc.stats.TotalMemoryBytes, 0)
}

func (mc *MetadataCache) Stop() {
	close(mc.stopChan)
}

func (mc *MetadataCache) buildKey(metaType MetadataType, key string) string {
	return fmt.Sprintf("%d:%s", metaType, key)
}

func (mc *MetadataCache) getTTLForType(metaType MetadataType) time.Duration {
	switch metaType {
	case MetadataCalendar:
		return mc.config.CalendarTTL
	case MetadataPrincipal:
		return mc.config.PrincipalTTL
	case MetadataHomeSet:
		return mc.config.HomeSetTTL
	case MetadataCalendarList:
		return mc.config.CalendarListTTL
	case MetadataPrivileges:
		return mc.config.PrivilegesTTL
	case MetadataQuota:
		return mc.config.QuotaTTL
	default:
		return 5 * time.Minute
	}
}

func (mc *MetadataCache) recordHit(metaType MetadataType) {
	atomic.AddInt64(&mc.stats.TotalHits, 1)

	switch metaType {
	case MetadataCalendar:
		atomic.AddInt64(&mc.stats.CalendarHits, 1)
	case MetadataPrincipal:
		atomic.AddInt64(&mc.stats.PrincipalHits, 1)
	case MetadataHomeSet:
		atomic.AddInt64(&mc.stats.HomeSetHits, 1)
	case MetadataCalendarList:
		atomic.AddInt64(&mc.stats.ListHits, 1)
	}
}

func (mc *MetadataCache) recordMiss(metaType MetadataType) {
	atomic.AddInt64(&mc.stats.TotalMisses, 1)

	switch metaType {
	case MetadataCalendar:
		atomic.AddInt64(&mc.stats.CalendarMisses, 1)
	case MetadataPrincipal:
		atomic.AddInt64(&mc.stats.PrincipalMisses, 1)
	case MetadataHomeSet:
		atomic.AddInt64(&mc.stats.HomeSetMisses, 1)
	case MetadataCalendarList:
		atomic.AddInt64(&mc.stats.ListMisses, 1)
	}
}

func (mc *MetadataCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range mc.entries {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(mc.entries, oldestKey)
		atomic.AddInt64(&mc.stats.Evictions, 1)
	}
}

func (mc *MetadataCache) estimateSize(data interface{}) int64 {
	switch v := data.(type) {
	case *Calendar:
		return 1024
	case []Calendar:
		return int64(len(v) * 1024)
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	default:
		return 512
	}
}

func (mc *MetadataCache) wouldExceedMemoryLimit(additionalSize int64) bool {
	currentMemory := atomic.LoadInt64(&mc.stats.TotalMemoryBytes)
	maxMemory := mc.config.MaxMemoryMB * 1024 * 1024
	return currentMemory+additionalSize > maxMemory
}

func (mc *MetadataCache) evictUntilMemoryAvailable(requiredSize int64) {
	targetMemory := (mc.config.MaxMemoryMB * 1024 * 1024) - requiredSize

	type entryInfo struct {
		key        string
		lastAccess time.Time
		size       int64
	}

	entries := make([]entryInfo, 0, len(mc.entries))
	for key, entry := range mc.entries {
		entries = append(entries, entryInfo{
			key:        key,
			lastAccess: entry.LastAccess,
			size:       entry.Size,
		})
	}

	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].lastAccess.After(entries[j].lastAccess) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	currentMemory := atomic.LoadInt64(&mc.stats.TotalMemoryBytes)
	for _, info := range entries {
		if currentMemory <= targetMemory {
			break
		}
		delete(mc.entries, info.key)
		currentMemory -= info.size
		atomic.AddInt64(&mc.stats.Evictions, 1)
	}
}

func (mc *MetadataCache) updateMemoryStats() {
	var totalMemory int64
	for _, entry := range mc.entries {
		totalMemory += entry.Size
	}
	atomic.StoreInt64(&mc.stats.TotalMemoryBytes, totalMemory)
}

func (mc *MetadataCache) refreshEntry(ctx context.Context, metaType MetadataType, key string) {
	callback, exists := mc.refreshCallbacks[metaType]
	if !exists || callback == nil {
		return
	}

	data, err := callback(ctx, key)
	if err == nil && data != nil {
		mc.Set(metaType, key, data, "")
		atomic.AddInt64(&mc.stats.RefreshCount, 1)
	}
}

func (mc *MetadataCache) startMaintenanceRoutine() {
	cleanupTicker := time.NewTicker(time.Minute)
	refreshTicker := time.NewTicker(mc.config.RefreshInterval)

	defer cleanupTicker.Stop()
	defer refreshTicker.Stop()

	for {
		select {
		case <-mc.stopChan:
			return
		case <-cleanupTicker.C:
			mc.cleanupExpired()
		case <-refreshTicker.C:
			if mc.config.EnableAutoRefresh {
				mc.refreshStaleEntries()
			}
		}
	}
}

func (mc *MetadataCache) cleanupExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, entry := range mc.entries {
		if now.After(entry.Expiry) {
			delete(mc.entries, key)
			atomic.AddInt64(&mc.stats.Evictions, 1)
		}
	}

	atomic.StoreInt64(&mc.stats.TotalEntries, int64(len(mc.entries)))
	mc.updateMemoryStats()
}

func (mc *MetadataCache) refreshStaleEntries() {
	mc.mu.RLock()
	staleEntries := make([]struct {
		Type MetadataType
		Key  string
	}, 0)

	now := time.Now()
	for fullKey, entry := range mc.entries {
		refreshThreshold := entry.Expiry.Add(-mc.config.RefreshInterval)
		if now.After(refreshThreshold) && entry.HitCount > 0 {
			key := fullKey[2:]
			staleEntries = append(staleEntries, struct {
				Type MetadataType
				Key  string
			}{entry.Type, key})
		}
	}
	mc.mu.RUnlock()

	ctx := context.Background()
	for _, stale := range staleEntries {
		go mc.refreshEntry(ctx, stale.Type, stale.Key)
	}
}

func (c *CalDAVClient) WithMetadataCache(cache *MetadataCache) *CalDAVClient {
	cache.client = c

	cache.SetRefreshCallback(MetadataCalendar, func(ctx context.Context, key string) (interface{}, error) {
		calendar, err := c.GetCalendarByPath(ctx, key)
		return calendar, err
	})

	cache.SetRefreshCallback(MetadataCalendarList, func(ctx context.Context, key string) (interface{}, error) {
		calendars, err := c.FindCalendars(ctx, key)
		return calendars, err
	})

	cache.SetRefreshCallback(MetadataPrincipal, func(ctx context.Context, key string) (interface{}, error) {
		principal, err := c.FindCurrentUserPrincipal(ctx)
		return principal, err
	})

	cache.SetRefreshCallback(MetadataHomeSet, func(ctx context.Context, key string) (interface{}, error) {
		homeSet, err := c.FindCalendarHomeSet(ctx, key)
		return homeSet, err
	})

	return c
}

func (c *CalDAVClient) GetCalendarByPath(ctx context.Context, path string) (*Calendar, error) {
	xmlBody, err := buildPropfindXML([]string{
		"displayname",
		"calendar-description",
		"calendar-color",
		"getctag",
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.propfind(ctx, path, "0", xmlBody)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(msResp.Responses) == 0 {
		return nil, fmt.Errorf("no calendar found at path: %s", path)
	}

	calendars := extractCalendarsFromResponse(msResp)
	if len(calendars) > 0 {
		return &calendars[0], nil
	}

	return nil, fmt.Errorf("calendar not found")
}
