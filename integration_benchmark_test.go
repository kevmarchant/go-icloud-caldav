//go:build integration
// +build integration

package caldav

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

func BenchmarkCreateEvent(b *testing.B) {
	client := getTestClient(&testing.T{})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		event := &CalendarObject{
			Summary:   fmt.Sprintf("Benchmark Event %d", i),
			StartTime: integrationTimePtr(time.Now().Add(24 * time.Hour)),
			EndTime:   integrationTimePtr(time.Now().Add(25 * time.Hour)),
		}

		err := client.CreateEvent(testCalendarPath, event)
		if err != nil {
			b.Fatalf("Failed to create event: %v", err)
		}

		_ = client.DeleteEventByUID(testCalendarPath, event.UID)
	}
}

func BenchmarkBatchCreateEvents(b *testing.B) {
	client := getTestClient(&testing.T{})
	ctx := context.Background()

	b.Run("Batch10", func(b *testing.B) {
		benchmarkBatchCreate(b, client, ctx, 10)
	})

	b.Run("Batch20", func(b *testing.B) {
		benchmarkBatchCreate(b, client, ctx, 20)
	})

	b.Run("Batch50", func(b *testing.B) {
		benchmarkBatchCreate(b, client, ctx, 50)
	})
}

func benchmarkBatchCreate(b *testing.B, client *CalDAVClient, ctx context.Context, batchSize int) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		events := make([]*CalendarObject, batchSize)
		for j := 0; j < batchSize; j++ {
			events[j] = &CalendarObject{
				Summary:   fmt.Sprintf("Batch Benchmark Event %d-%d", i, j),
				StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+j) * time.Hour)),
				EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+j) * time.Hour)),
			}
		}

		responses, err := client.BatchCreateEvents(ctx, testCalendarPath, events)
		if err != nil {
			b.Fatalf("Batch create failed: %v", err)
		}

		eventPaths := make([]string, 0, len(responses))
		for k, resp := range responses {
			if resp.Success && events[k].UID != "" {
				eventPaths = append(eventPaths, fmt.Sprintf("%s%s.ics", testCalendarPath, events[k].UID))
			}
		}

		if len(eventPaths) > 0 {
			_, _ = client.BatchDeleteEvents(ctx, eventPaths)
		}
	}
}

func BenchmarkCacheHitRate(b *testing.B) {
	client := getTestClientWithCache(&testing.T{})
	ctx := context.Background()

	metaCache := NewMetadataCache(DefaultMetadataCacheConfig())
	client.WithMetadataCache(metaCache)

	calendar, err := client.GetCalendarByPath(ctx, testCalendarPath)
	if err != nil {
		b.Fatalf("Failed to get calendar: %v", err)
	}

	metaCache.Set(MetadataCalendar, testCalendarPath, calendar, "")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, found := metaCache.Get(ctx, MetadataCalendar, testCalendarPath)
		if !found {
			b.Fatal("Cache miss when hit was expected")
		}
	}

	b.StopTimer()

	stats := metaCache.GetStats()
	b.Logf("Cache hit rate: %.2f%%", metaCache.GetHitRate(MetadataCalendar))
	b.Logf("Total hits: %d", stats.TotalHits)
}

func BenchmarkBatchProcessorWithWorkers(b *testing.B) {
	client := getTestClient(&testing.T{})
	ctx := context.Background()

	workerCounts := []int{1, 5, 10, 20}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers%d", workers), func(b *testing.B) {
			processor := client.NewCRUDBatchProcessor(
				WithMaxWorkers(workers),
				WithTimeout(30*time.Second),
			)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				requests := make([]BatchCRUDRequest, 20)
				for j := 0; j < 20; j++ {
					requests[j] = BatchCRUDRequest{
						Operation:    OpCreate,
						CalendarPath: testCalendarPath,
						Event: &CalendarObject{
							Summary:   fmt.Sprintf("Worker Benchmark %d-%d", i, j),
							StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+j) * time.Hour)),
							EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+j) * time.Hour)),
						},
						RequestID: fmt.Sprintf("bench-%d-%d", i, j),
					}
				}

				responses, err := processor.ExecuteBatch(ctx, requests)
				if err != nil {
					b.Fatalf("Batch execution failed: %v", err)
				}

				for k, resp := range responses {
					if resp.Success && requests[k].Event.UID != "" {
						_ = client.DeleteEventByUID(testCalendarPath, requests[k].Event.UID)
					}
				}
			}

			b.StopTimer()

			metrics := processor.GetMetrics()
			b.Logf("Workers: %d, Success rate: %.2f%%, Avg duration: %v",
				workers, metrics.SuccessRate(), metrics.AverageDuration())
		})
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	client := getTestClientWithCache(&testing.T{})
	ctx := context.Background()

	b.Run("WithoutCache", func(b *testing.B) {
		benchmarkMemoryWithoutCache(b, client, ctx)
	})

	b.Run("WithCache", func(b *testing.B) {
		benchmarkMemoryWithCache(b, client, ctx)
	})
}

func benchmarkMemoryWithoutCache(b *testing.B, client *CalDAVClient, ctx context.Context) {
	var m runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m)
	startAlloc := m.Alloc

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			_, _ = client.GetCalendarByPath(ctx, testCalendarPath)
		}
	}

	b.StopTimer()

	runtime.GC()
	runtime.ReadMemStats(&m)
	endAlloc := m.Alloc

	b.Logf("Memory used without cache: %d KB", (endAlloc-startAlloc)/1024)
}

func benchmarkMemoryWithCache(b *testing.B, client *CalDAVClient, ctx context.Context) {
	metaCache := NewMetadataCache(&MetadataCacheConfig{
		CalendarTTL: 5 * time.Minute,
		MaxEntries:  100,
		MaxMemoryMB: 10,
	})
	client.WithMetadataCache(metaCache)

	var m runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m)
	startAlloc := m.Alloc

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			calendar, _ := client.GetCalendarByPath(ctx, testCalendarPath)
			if j == 0 {
				metaCache.Set(MetadataCalendar, testCalendarPath, calendar, "")
			} else {
				_, _ = metaCache.Get(ctx, MetadataCalendar, testCalendarPath)
			}
		}
	}

	b.StopTimer()

	runtime.GC()
	runtime.ReadMemStats(&m)
	endAlloc := m.Alloc

	stats := metaCache.GetStats()
	b.Logf("Memory used with cache: %d KB", (endAlloc-startAlloc)/1024)
	b.Logf("Cache memory: %d KB", stats.TotalMemoryBytes/1024)
	b.Logf("Cache hit rate: %.2f%%", metaCache.GetHitRate(MetadataCalendar))
}

func BenchmarkConnectionPooling(b *testing.B) {
	b.Run("WithPooling", func(b *testing.B) {
		client := getTestClient(&testing.T{})
		ctx := context.Background()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				_, _ = client.FindCurrentUserPrincipal(ctx)
			}
		}

		b.StopTimer()

		if client.connectionMetrics != nil {
			metrics := client.GetConnectionMetrics()
			reuseRate := float64(metrics.ConnectionReuses) / float64(metrics.TotalRequests) * 100
			http2Rate := float64(metrics.HTTP2Connections) / float64(metrics.TotalRequests) * 100
			b.Logf("Connection reuse rate: %.2f%%", reuseRate)
			b.Logf("HTTP/2 usage: %.2f%%", http2Rate)
		}
	})

	b.Run("WithoutPooling", func(b *testing.B) {
		username := getEnvOrSkip(b, "ICLOUD_USERNAME")
		password := getEnvOrSkip(b, "ICLOUD_PASSWORD")

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for j := 0; j < 10; j++ {
				client := NewClient(username, password)
				_, _ = client.FindCurrentUserPrincipal(context.Background())
			}
		}
	})
}

func getEnvOrSkip(b *testing.B, key string) string {
	value := os.Getenv(key)
	if value == "" {
		b.Skipf("%s environment variable not set", key)
	}
	return value
}

func BenchmarkConcurrentOperations(b *testing.B) {
	client := getTestClient(&testing.T{})
	ctx := context.Background()
	_ = ctx

	concurrencyLevels := []int{1, 5, 10, 20}

	for _, level := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", level), func(b *testing.B) {
			b.SetParallelism(level)

			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					event := &CalendarObject{
						Summary:   fmt.Sprintf("Concurrent Benchmark %d", i),
						StartTime: integrationTimePtr(time.Now().Add(24 * time.Hour)),
						EndTime:   integrationTimePtr(time.Now().Add(25 * time.Hour)),
					}

					err := client.CreateEvent(testCalendarPath, event)
					if err == nil {
						_ = client.DeleteEventByUID(testCalendarPath, event.UID)
					}
					i++
				}
			})
		})
	}
}

func BenchmarkQueryCalendar(b *testing.B) {
	client := getTestClient(&testing.T{})
	ctx := context.Background()

	events := make([]*CalendarObject, 50)
	for i := 0; i < 50; i++ {
		events[i] = &CalendarObject{
			Summary:   fmt.Sprintf("Query Benchmark Event %d", i),
			StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+i) * time.Hour)),
			EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+i) * time.Hour)),
		}
		_ = client.CreateEvent(testCalendarPath, events[i])
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		query := CalendarQuery{
			Filter: Filter{
				Component: "VCALENDAR",
				CompFilters: []Filter{
					{
						Component: "VEVENT",
						TimeRange: &TimeRange{
							Start: time.Now(),
							End:   time.Now().Add(7 * 24 * time.Hour),
						},
					},
				},
			},
		}

		_, _ = client.QueryCalendar(ctx, testCalendarPath, query)
	}

	b.StopTimer()

	for _, event := range events {
		if event.UID != "" {
			_ = client.DeleteEventByUID(testCalendarPath, event.UID)
		}
	}
}

func integrationTimePtr(t time.Time) *time.Time {
	return &t
}
