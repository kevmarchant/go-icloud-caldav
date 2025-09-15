//go:build integration
// +build integration

package caldav

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

const testCalendarPath = "/182969443/calendars/254A20C4-1E6D-45BF-819F-94E5E5823AC5/"

func getTestClientWithCache(t *testing.T) *CalDAVClient {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")

	if username == "" || password == "" {
		t.Skip("ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables must be set for integration tests")
	}

	client := NewClient(username, password)

	cache := NewResponseCache(5*time.Minute, 100)
	client.WithCache(cache)

	metaCache := NewMetadataCache(DefaultMetadataCacheConfig())
	client.WithMetadataCache(metaCache)

	return client
}

func TestIntegrationCRUDLifecycle(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	event := &CalendarObject{
		Summary:     "Integration Test Event",
		Description: "This is a test event created by integration tests",
		StartTime:   integrationTimePtr(time.Now().Add(24 * time.Hour)),
		EndTime:     integrationTimePtr(time.Now().Add(25 * time.Hour)),
		Location:    "Test Location",
	}

	t.Run("CreateEvent", func(t *testing.T) {
		err := client.CreateEvent(testCalendarPath, event)
		if err != nil {
			t.Fatalf("Failed to create event: %v", err)
		}
		t.Logf("Created event with UID: %s", event.UID)
	})

	eventUID := event.UID

	t.Run("VerifyEventExists", func(t *testing.T) {
		events, err := client.QueryCalendar(ctx, testCalendarPath, CalendarQuery{
			Filter: Filter{
				Component: "VCALENDAR",
				CompFilters: []Filter{
					{
						Component: "VEVENT",
						Props: []PropFilter{
							{
								Name: "UID",
								TextMatch: &TextMatch{
									Value: eventUID,
								},
							},
						},
					},
				},
			},
		})

		if err != nil {
			t.Fatalf("Failed to query calendar: %v", err)
		}

		if len(events) == 0 {
			t.Fatal("Event not found after creation")
		}

		if events[0].Summary != event.Summary {
			t.Errorf("Summary mismatch: expected %s, got %s", event.Summary, events[0].Summary)
		}
	})

	t.Run("UpdateEvent", func(t *testing.T) {
		event.Summary = "Updated Integration Test Event"
		event.Description = "This event has been updated"
		event.Location = "Updated Location"

		err := client.UpdateEvent(testCalendarPath, event, "")
		if err != nil {
			t.Fatalf("Failed to update event: %v", err)
		}
		t.Log("Event updated successfully")
	})

	t.Run("VerifyEventUpdated", func(t *testing.T) {
		events, err := client.QueryCalendar(ctx, testCalendarPath, CalendarQuery{
			Filter: Filter{
				Component: "VCALENDAR",
				CompFilters: []Filter{
					{
						Component: "VEVENT",
						Props: []PropFilter{
							{
								Name: "UID",
								TextMatch: &TextMatch{
									Value: eventUID,
								},
							},
						},
					},
				},
			},
		})

		if err != nil {
			t.Fatalf("Failed to query calendar: %v", err)
		}

		if len(events) == 0 {
			t.Fatal("Event not found after update")
		}

		if events[0].Summary != "Updated Integration Test Event" {
			t.Errorf("Summary not updated: got %s", events[0].Summary)
		}

		if events[0].Location != "Updated Location" {
			t.Errorf("Location not updated: got %s", events[0].Location)
		}
	})

	t.Run("DeleteEvent", func(t *testing.T) {
		err := client.DeleteEventByUID(testCalendarPath, eventUID)
		if err != nil {
			t.Fatalf("Failed to delete event: %v", err)
		}
		t.Log("Event deleted successfully")
	})

	t.Run("VerifyEventDeleted", func(t *testing.T) {
		events, err := client.QueryCalendar(ctx, testCalendarPath, CalendarQuery{
			Filter: Filter{
				Component: "VCALENDAR",
				CompFilters: []Filter{
					{
						Component: "VEVENT",
						Props: []PropFilter{
							{
								Name: "UID",
								TextMatch: &TextMatch{
									Value: eventUID,
								},
							},
						},
					},
				},
			},
		})

		if err != nil {
			t.Fatalf("Failed to query calendar: %v", err)
		}

		if len(events) != 0 {
			t.Error("Event still exists after deletion")
		}
	})
}

func TestIntegrationConcurrentCRUD(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	numEvents := 5
	var wg sync.WaitGroup
	errors := make(chan error, numEvents*3)

	t.Run("ConcurrentCreate", func(t *testing.T) {
		for i := 0; i < numEvents; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				event := &CalendarObject{
					Summary:   fmt.Sprintf("Concurrent Event %d", index),
					StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+index) * time.Hour)),
					EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+index) * time.Hour)),
				}

				err := client.CreateEvent(testCalendarPath, event)
				if err != nil {
					errors <- fmt.Errorf("create event %d: %w", index, err)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Error(err)
		}
	})

	t.Run("CleanupConcurrentEvents", func(t *testing.T) {
		events, err := client.GetRecentEvents(ctx, testCalendarPath, 30)
		if err != nil {
			t.Logf("Warning: Failed to get events for cleanup: %v", err)
			return
		}

		for _, event := range events {
			if len(event.Summary) >= 16 && event.Summary[:16] == "Concurrent Event" {
				err := client.DeleteEventByUID(testCalendarPath, event.UID)
				if err != nil {
					t.Logf("Warning: Failed to delete event %s: %v", event.UID, err)
				}
			}
		}
	})
}

func TestIntegrationBatchCRUD(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	t.Run("BatchCreate", func(t *testing.T) {
		events := make([]*CalendarObject, 10)
		for i := 0; i < 10; i++ {
			events[i] = &CalendarObject{
				Summary:   fmt.Sprintf("Batch Event %d", i),
				StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+i) * time.Hour)),
				EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+i) * time.Hour)),
				Location:  fmt.Sprintf("Location %d", i),
			}
		}

		startTime := time.Now()
		responses, err := client.BatchCreateEvents(ctx, testCalendarPath, events)
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Batch create failed: %v", err)
		}

		successCount := 0
		for i, resp := range responses {
			if resp.Success {
				successCount++
				t.Logf("Event %d created successfully", i)
			} else {
				t.Errorf("Event %d failed: %v", i, resp.Error)
			}
		}

		t.Logf("Batch created %d/%d events in %v", successCount, len(events), duration)

		if successCount != len(events) {
			t.Errorf("Not all events created successfully: %d/%d", successCount, len(events))
		}
	})

	var createdUIDs []string

	t.Run("VerifyBatchCreated", func(t *testing.T) {
		events, err := client.GetRecentEvents(ctx, testCalendarPath, 30)
		if err != nil {
			t.Fatalf("Failed to get events: %v", err)
		}

		for _, event := range events {
			if len(event.Summary) >= 11 && event.Summary[:11] == "Batch Event" {
				createdUIDs = append(createdUIDs, event.UID)
			}
		}

		if len(createdUIDs) < 10 {
			t.Errorf("Expected at least 10 batch events, found %d", len(createdUIDs))
		}
	})

	t.Run("BatchUpdate", func(t *testing.T) {
		if len(createdUIDs) == 0 {
			t.Skip("No events to update")
		}

		updates := make([]struct {
			Event *CalendarObject
			ETag  string
		}, len(createdUIDs))

		for i, uid := range createdUIDs {
			updates[i] = struct {
				Event *CalendarObject
				ETag  string
			}{
				Event: &CalendarObject{
					UID:       uid,
					Summary:   fmt.Sprintf("Updated Batch Event %d", i),
					StartTime: integrationTimePtr(time.Now().Add(time.Duration(48+i) * time.Hour)),
					EndTime:   integrationTimePtr(time.Now().Add(time.Duration(49+i) * time.Hour)),
				},
				ETag: "",
			}
		}

		startTime := time.Now()
		responses, err := client.BatchUpdateEvents(ctx, testCalendarPath, updates)
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Batch update failed: %v", err)
		}

		successCount := 0
		for _, resp := range responses {
			if resp.Success {
				successCount++
			}
		}

		t.Logf("Batch updated %d/%d events in %v", successCount, len(updates), duration)
	})

	t.Run("BatchDelete", func(t *testing.T) {
		if len(createdUIDs) == 0 {
			t.Skip("No events to delete")
		}

		eventPaths := make([]string, len(createdUIDs))
		for i, uid := range createdUIDs {
			eventPaths[i] = fmt.Sprintf("%s%s.ics", testCalendarPath, uid)
		}

		startTime := time.Now()
		responses, err := client.BatchDeleteEvents(ctx, eventPaths)
		duration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Batch delete failed: %v", err)
		}

		successCount := 0
		for _, resp := range responses {
			if resp.Success {
				successCount++
			}
		}

		t.Logf("Batch deleted %d/%d events in %v", successCount, len(eventPaths), duration)
	})
}

func TestIntegrationBatchMetrics(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	processor := client.NewCRUDBatchProcessor(
		WithMaxWorkers(5),
		WithTimeout(30*time.Second),
	)

	events := make([]*CalendarObject, 20)
	for i := 0; i < 20; i++ {
		events[i] = &CalendarObject{
			Summary:   fmt.Sprintf("Metrics Test Event %d", i),
			StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+i) * time.Hour)),
			EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+i) * time.Hour)),
		}
	}

	requests := make([]BatchCRUDRequest, len(events))
	for i, event := range events {
		requests[i] = BatchCRUDRequest{
			Operation:    OpCreate,
			CalendarPath: testCalendarPath,
			Event:        event,
			RequestID:    fmt.Sprintf("metrics-test-%d", i),
		}
	}

	responses, err := processor.ExecuteBatch(ctx, requests)
	if err != nil {
		t.Fatalf("Batch execution failed: %v", err)
	}

	metrics := processor.GetMetrics()

	t.Logf("Batch Metrics:")
	t.Logf("  Total Requests: %d", metrics.TotalRequests)
	t.Logf("  Successful: %d", metrics.SuccessfulOps)
	t.Logf("  Failed: %d", metrics.FailedOps)
	t.Logf("  Success Rate: %.2f%%", metrics.SuccessRate())
	t.Logf("  Average Duration: %v", metrics.AverageDuration())
	t.Logf("  Fastest: %v", metrics.FastestDuration())
	t.Logf("  Slowest: %v", metrics.SlowestDuration())

	if metrics.SuccessRate() < 90 {
		t.Errorf("Success rate too low: %.2f%%", metrics.SuccessRate())
	}

	createdUIDs := make([]string, 0, len(responses))
	for _, resp := range responses {
		if resp.Success {
			for _, event := range events {
				if event.UID != "" {
					createdUIDs = append(createdUIDs, event.UID)
					break
				}
			}
		}
	}

	t.Run("CleanupMetricsEvents", func(t *testing.T) {
		for _, uid := range createdUIDs {
			_ = client.DeleteEventByUID(testCalendarPath, uid)
		}

		events, err := client.GetRecentEvents(ctx, testCalendarPath, 50)
		if err == nil {
			for _, event := range events {
				if len(event.Summary) >= 18 && event.Summary[:18] == "Metrics Test Event" {
					_ = client.DeleteEventByUID(testCalendarPath, event.UID)
				}
			}
		}
	})
}

func TestIntegrationMetadataCache(t *testing.T) {
	client := getTestClientWithCache(t)
	ctx := context.Background()

	metaCache := NewMetadataCache(DefaultMetadataCacheConfig())
	client.WithMetadataCache(metaCache)

	t.Run("CalendarMetadataCaching", func(t *testing.T) {
		startTime := time.Now()
		calendar1, err := client.GetCalendarByPath(ctx, testCalendarPath)
		firstCallDuration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Failed to get calendar: %v", err)
		}

		metaCache.Set(MetadataCalendar, testCalendarPath, calendar1, "")

		startTime = time.Now()
		calendar2, found := metaCache.Get(ctx, MetadataCalendar, testCalendarPath)
		secondCallDuration := time.Since(startTime)

		if !found {
			t.Error("Expected calendar to be cached")
		}

		if calendar2 == nil {
			t.Error("Cached calendar is nil")
		}

		t.Logf("First call (no cache): %v", firstCallDuration)
		t.Logf("Second call (cached): %v", secondCallDuration)

		if secondCallDuration >= firstCallDuration {
			t.Error("Cached call should be faster than uncached call")
		}

		stats := metaCache.GetStats()
		t.Logf("Cache Stats:")
		t.Logf("  Total Hits: %d", stats.TotalHits)
		t.Logf("  Total Misses: %d", stats.TotalMisses)
		t.Logf("  Hit Rate: %.2f%%", metaCache.GetHitRate(MetadataCalendar))
	})

	t.Run("CacheInvalidation", func(t *testing.T) {
		event := &CalendarObject{
			Summary:   "Cache Test Event",
			StartTime: integrationTimePtr(time.Now().Add(24 * time.Hour)),
			EndTime:   integrationTimePtr(time.Now().Add(25 * time.Hour)),
		}

		err := client.CreateEvent(testCalendarPath, event)
		if err != nil {
			t.Fatalf("Failed to create event: %v", err)
		}

		metaCache.InvalidateKey(MetadataCalendar, testCalendarPath)

		_, found := metaCache.Get(ctx, MetadataCalendar, testCalendarPath)
		if found {
			t.Error("Expected cache to be invalidated")
		}

		err = client.DeleteEventByUID(testCalendarPath, event.UID)
		if err != nil {
			t.Logf("Warning: Failed to delete test event: %v", err)
		}
	})
}

func TestIntegrationPerformanceComparison(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	numEvents := 20

	t.Run("SequentialOperations", func(t *testing.T) {
		events := make([]*CalendarObject, numEvents)
		for i := 0; i < numEvents; i++ {
			events[i] = &CalendarObject{
				Summary:   fmt.Sprintf("Sequential Event %d", i),
				StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+i) * time.Hour)),
				EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+i) * time.Hour)),
			}
		}

		startTime := time.Now()
		for _, event := range events {
			err := client.CreateEvent(testCalendarPath, event)
			if err != nil {
				t.Errorf("Failed to create event: %v", err)
			}
		}
		sequentialDuration := time.Since(startTime)

		t.Logf("Sequential create of %d events: %v", numEvents, sequentialDuration)
		t.Logf("Average per event: %v", sequentialDuration/time.Duration(numEvents))

		for _, event := range events {
			_ = client.DeleteEventByUID(testCalendarPath, event.UID)
		}
	})

	t.Run("BatchOperations", func(t *testing.T) {
		events := make([]*CalendarObject, numEvents)
		for i := 0; i < numEvents; i++ {
			events[i] = &CalendarObject{
				Summary:   fmt.Sprintf("Batch Perf Event %d", i),
				StartTime: integrationTimePtr(time.Now().Add(time.Duration(24+i) * time.Hour)),
				EndTime:   integrationTimePtr(time.Now().Add(time.Duration(25+i) * time.Hour)),
			}
		}

		startTime := time.Now()
		responses, err := client.BatchCreateEvents(ctx, testCalendarPath, events)
		batchDuration := time.Since(startTime)

		if err != nil {
			t.Fatalf("Batch create failed: %v", err)
		}

		successCount := 0
		for _, resp := range responses {
			if resp.Success {
				successCount++
			}
		}

		t.Logf("Batch create of %d events: %v", numEvents, batchDuration)
		t.Logf("Average per event: %v", batchDuration/time.Duration(numEvents))
		t.Logf("Success rate: %.2f%%", float64(successCount)/float64(numEvents)*100)

		eventPaths := make([]string, 0, len(events))
		for _, event := range events {
			if event.UID != "" {
				eventPaths = append(eventPaths, fmt.Sprintf("%s%s.ics", testCalendarPath, event.UID))
			}
		}

		if len(eventPaths) > 0 {
			_, _ = client.BatchDeleteEvents(ctx, eventPaths)
		}
	})
}
