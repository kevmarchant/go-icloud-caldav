package caldav

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMetadataCache_Basic(t *testing.T) {
	config := &MetadataCacheConfig{
		CalendarTTL: time.Minute,
		MaxEntries:  10,
		MaxMemoryMB: 1,
	}

	cache := NewMetadataCache(config)
	defer cache.Stop()

	ctx := context.Background()

	calendar := &Calendar{
		Name:        "Test Calendar",
		Href:        "/calendars/test/",
		Description: "Test Description",
		Color:       "#FF0000",
	}

	cache.Set(MetadataCalendar, "/calendars/test/", calendar, "etag-123")

	data, found := cache.Get(ctx, MetadataCalendar, "/calendars/test/")
	if !found {
		t.Error("Expected to find cached calendar")
	}

	cachedCal, ok := data.(*Calendar)
	if !ok {
		t.Error("Expected cached data to be *Calendar")
	}

	if cachedCal.Name != calendar.Name {
		t.Errorf("Expected calendar name %s, got %s", calendar.Name, cachedCal.Name)
	}

	stats := cache.GetStats()
	if stats.CalendarHits != 1 {
		t.Errorf("Expected 1 calendar hit, got %d", stats.CalendarHits)
	}
}

func TestMetadataCache_TTLExpiry(t *testing.T) {
	config := &MetadataCacheConfig{
		CalendarTTL: 10 * time.Millisecond,
		MaxEntries:  10,
	}

	cache := NewMetadataCache(config)
	defer cache.Stop()

	ctx := context.Background()

	cache.Set(MetadataCalendar, "test-key", "test-data", "")

	data, found := cache.Get(ctx, MetadataCalendar, "test-key")
	if !found {
		t.Error("Expected to find cached data immediately")
	}

	if data != "test-data" {
		t.Errorf("Expected 'test-data', got %v", data)
	}

	time.Sleep(20 * time.Millisecond)

	_, found = cache.Get(ctx, MetadataCalendar, "test-key")
	if found {
		t.Error("Expected cached data to be expired")
	}

	stats := cache.GetStats()
	if stats.CalendarMisses != 1 {
		t.Errorf("Expected 1 calendar miss, got %d", stats.CalendarMisses)
	}
}

func TestMetadataCache_DifferentTypes(t *testing.T) {
	config := &MetadataCacheConfig{
		CalendarTTL:     time.Minute,
		PrincipalTTL:    2 * time.Minute,
		HomeSetTTL:      3 * time.Minute,
		CalendarListTTL: time.Minute,
		MaxEntries:      100,
		MaxMemoryMB:     100,
	}

	cache := NewMetadataCache(config)
	defer cache.Stop()

	ctx := context.Background()

	cache.Set(MetadataCalendar, "cal-1", &Calendar{Name: "Calendar 1"}, "")
	cache.Set(MetadataPrincipal, "principal-1", "/principals/user/", "")
	cache.Set(MetadataHomeSet, "homeset-1", "/calendars/user/", "")
	cache.Set(MetadataCalendarList, "list-1", []Calendar{{Name: "Cal1"}, {Name: "Cal2"}}, "")

	if _, found := cache.Get(ctx, MetadataCalendar, "cal-1"); !found {
		t.Error("Expected to find calendar")
	}

	if _, found := cache.Get(ctx, MetadataPrincipal, "principal-1"); !found {
		t.Error("Expected to find principal")
	}

	if _, found := cache.Get(ctx, MetadataHomeSet, "homeset-1"); !found {
		t.Error("Expected to find homeset")
	}

	if _, found := cache.Get(ctx, MetadataCalendarList, "list-1"); !found {
		t.Error("Expected to find calendar list")
	}

	stats := cache.GetStats()
	if stats.TotalHits != 4 {
		t.Errorf("Expected 4 total hits, got %d", stats.TotalHits)
	}

	if stats.TotalEntries != 4 {
		t.Errorf("Expected 4 total entries, got %d", stats.TotalEntries)
	}
}

func TestMetadataCache_InvalidateType(t *testing.T) {
	cache := NewMetadataCache(nil)
	defer cache.Stop()

	cache.Set(MetadataCalendar, "cal-1", "data1", "")
	cache.Set(MetadataCalendar, "cal-2", "data2", "")
	cache.Set(MetadataCalendar, "cal-3", "data3", "")
	cache.Set(MetadataPrincipal, "principal-1", "principal-data", "")

	count := cache.InvalidateType(MetadataCalendar)
	if count != 3 {
		t.Errorf("Expected to invalidate 3 calendars, got %d", count)
	}

	ctx := context.Background()
	_, found := cache.Get(ctx, MetadataCalendar, "cal-1")
	if found {
		t.Error("Expected calendar to be invalidated")
	}

	_, found = cache.Get(ctx, MetadataPrincipal, "principal-1")
	if !found {
		t.Error("Expected principal to still be cached")
	}

	stats := cache.GetStats()
	if stats.TotalEntries != 1 {
		t.Errorf("Expected 1 remaining entry, got %d", stats.TotalEntries)
	}
}

func TestMetadataCache_InvalidateKey(t *testing.T) {
	cache := NewMetadataCache(nil)
	defer cache.Stop()

	cache.Set(MetadataCalendar, "cal-1", "data1", "")
	cache.Set(MetadataCalendar, "cal-2", "data2", "")

	removed := cache.InvalidateKey(MetadataCalendar, "cal-1")
	if !removed {
		t.Error("Expected to remove cal-1")
	}

	ctx := context.Background()
	_, found := cache.Get(ctx, MetadataCalendar, "cal-1")
	if found {
		t.Error("Expected cal-1 to be invalidated")
	}

	_, found = cache.Get(ctx, MetadataCalendar, "cal-2")
	if !found {
		t.Error("Expected cal-2 to still be cached")
	}
}

func TestMetadataCache_HitRate(t *testing.T) {
	cache := NewMetadataCache(nil)
	defer cache.Stop()

	ctx := context.Background()

	cache.Set(MetadataCalendar, "cal-1", "data", "")

	for i := 0; i < 7; i++ {
		cache.Get(ctx, MetadataCalendar, "cal-1")
	}

	for i := 0; i < 3; i++ {
		cache.Get(ctx, MetadataCalendar, "non-existent")
	}

	hitRate := cache.GetHitRate(MetadataCalendar)
	expectedRate := 70.0

	if hitRate != expectedRate {
		t.Errorf("Expected hit rate %.1f%%, got %.1f%%", expectedRate, hitRate)
	}

	totalHitRate := cache.GetHitRate(MetadataType(-1))
	if totalHitRate != expectedRate {
		t.Errorf("Expected total hit rate %.1f%%, got %.1f%%", expectedRate, totalHitRate)
	}
}

func TestMetadataCache_MemoryLimits(t *testing.T) {
	config := &MetadataCacheConfig{
		CalendarTTL: time.Minute,
		MaxEntries:  100,
		MaxMemoryMB: 1,
	}

	cache := NewMetadataCache(config)
	defer cache.Stop()

	for i := 0; i < 50; i++ {
		largeCalendarList := make([]Calendar, 100)
		for j := 0; j < 100; j++ {
			largeCalendarList[j] = Calendar{
				Name:        fmt.Sprintf("Calendar %d-%d", i, j),
				Href:        fmt.Sprintf("/calendars/test-%d-%d/", i, j),
				Description: "Large calendar for memory testing",
			}
		}
		cache.Set(MetadataCalendarList, fmt.Sprintf("list-%d", i), largeCalendarList, "")
	}

	stats := cache.GetStats()
	memoryMB := stats.TotalMemoryBytes / (1024 * 1024)

	if memoryMB > config.MaxMemoryMB {
		t.Errorf("Memory usage %d MB exceeds limit %d MB", memoryMB, config.MaxMemoryMB)
	}

	if stats.Evictions == 0 {
		t.Error("Expected some evictions due to memory limit")
	}
}

func TestMetadataCache_RefreshCallback(t *testing.T) {
	config := &MetadataCacheConfig{
		CalendarTTL:       50 * time.Millisecond,
		EnableAutoRefresh: false,
	}

	cache := NewMetadataCache(config)
	defer cache.Stop()

	refreshCount := 0
	cache.SetRefreshCallback(MetadataCalendar, func(ctx context.Context, key string) (interface{}, error) {
		refreshCount++
		return &Calendar{Name: fmt.Sprintf("Refreshed-%d", refreshCount)}, nil
	})

	ctx := context.Background()
	cache.Set(MetadataCalendar, "cal-1", &Calendar{Name: "Original"}, "")

	data, found := cache.Get(ctx, MetadataCalendar, "cal-1")
	if !found {
		t.Fatal("Expected to find calendar")
	}

	cal := data.(*Calendar)
	if cal.Name != "Original" {
		t.Errorf("Expected 'Original', got %s", cal.Name)
	}

	time.Sleep(60 * time.Millisecond)

	_, found = cache.Get(ctx, MetadataCalendar, "cal-1")
	if found {
		t.Error("Expected cache to expire")
	}
}

func TestMetadataCache_Clear(t *testing.T) {
	cache := NewMetadataCache(nil)
	defer cache.Stop()

	for i := 0; i < 10; i++ {
		cache.Set(MetadataCalendar, fmt.Sprintf("cal-%d", i), fmt.Sprintf("data-%d", i), "")
	}

	stats := cache.GetStats()
	if stats.TotalEntries != 10 {
		t.Errorf("Expected 10 entries, got %d", stats.TotalEntries)
	}

	cache.Clear()

	stats = cache.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", stats.TotalEntries)
	}

	if stats.Evictions != 10 {
		t.Errorf("Expected 10 evictions, got %d", stats.Evictions)
	}
}

func TestMetadataCache_LRUEviction(t *testing.T) {
	config := &MetadataCacheConfig{
		CalendarTTL: time.Minute,
		MaxEntries:  3,
		MaxMemoryMB: 100,
	}

	cache := NewMetadataCache(config)
	defer cache.Stop()

	ctx := context.Background()

	cache.Set(MetadataCalendar, "cal-1", "data-1", "")
	time.Sleep(10 * time.Millisecond)

	cache.Set(MetadataCalendar, "cal-2", "data-2", "")
	time.Sleep(10 * time.Millisecond)

	cache.Set(MetadataCalendar, "cal-3", "data-3", "")

	cache.Get(ctx, MetadataCalendar, "cal-1")
	cache.Get(ctx, MetadataCalendar, "cal-2")

	cache.Set(MetadataCalendar, "cal-4", "data-4", "")

	_, found := cache.Get(ctx, MetadataCalendar, "cal-3")
	if found {
		t.Error("Expected cal-3 to be evicted (least recently used)")
	}

	_, found = cache.Get(ctx, MetadataCalendar, "cal-1")
	if !found {
		t.Error("Expected cal-1 to still be cached")
	}

	_, found = cache.Get(ctx, MetadataCalendar, "cal-2")
	if !found {
		t.Error("Expected cal-2 to still be cached")
	}

	_, found = cache.Get(ctx, MetadataCalendar, "cal-4")
	if !found {
		t.Error("Expected cal-4 to be cached")
	}
}
