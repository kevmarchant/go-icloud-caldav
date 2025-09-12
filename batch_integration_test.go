//go:build integration
// +build integration

package caldav

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParallelCalendarSyncIntegration(t *testing.T) {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")
	if username == "" || password == "" {
		t.Skip("ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables not set")
	}

	client := NewClientWithOptions(username, password)

	ctx := context.Background()

	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		t.Fatalf("Failed to find current user principal: %v", err)
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		t.Fatalf("Failed to find calendar home set: %v", err)
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		t.Fatalf("Failed to find calendars: %v", err)
	}

	if len(calendars) == 0 {
		t.Skip("No calendars found")
	}

	t.Logf("Found %d calendars to sync", len(calendars))

	calendarPaths := make([]string, len(calendars))
	for i, cal := range calendars {
		calendarPaths[i] = cal.Href
		t.Logf("Calendar %d: %s", i+1, cal.DisplayName)
	}

	t.Run("Serial_vs_Parallel_Comparison", func(t *testing.T) {
		days := 30

		serialStart := time.Now()
		var serialTotal int
		for _, calPath := range calendarPaths {
			events, err := client.GetRecentEvents(ctx, calPath, days)
			if err != nil {
				t.Logf("Error fetching events from %s: %v", calPath, err)
				continue
			}
			serialTotal += len(events)
		}
		serialDuration := time.Since(serialStart)

		t.Logf("Serial sync: %d events in %v", serialTotal, serialDuration)

		parallelStart := time.Now()
		config := &BatchQueryConfig{
			MaxConcurrency: 5,
			Timeout:        30 * time.Second,
		}
		results := client.GetRecentEventsParallel(ctx, calendarPaths, days, config)

		parallelTotal := 0
		failedCalendars := 0
		for _, result := range results {
			if result.Error != nil {
				t.Logf("Error in parallel fetch for %s: %v", result.CalendarPath, result.Error)
				failedCalendars++
			} else {
				parallelTotal += len(result.Objects)
			}
		}
		parallelDuration := time.Since(parallelStart)

		t.Logf("Parallel sync (5 workers): %d events in %v", parallelTotal, parallelDuration)

		if failedCalendars > 0 {
			t.Logf("Failed calendars: %d/%d", failedCalendars, len(calendars))
		}

		speedup := float64(serialDuration) / float64(parallelDuration)
		t.Logf("Speed improvement: %.2fx faster", speedup)

		if speedup < 1.5 && len(calendars) > 3 {
			t.Logf("Warning: Expected better speedup for %d calendars, got %.2fx", len(calendars), speedup)
		}
	})

	t.Run("Different_Concurrency_Levels", func(t *testing.T) {
		days := 7
		concurrencyLevels := []int{1, 3, 5, 10}

		for _, concurrency := range concurrencyLevels {
			if concurrency > len(calendars) {
				concurrency = len(calendars)
			}

			start := time.Now()
			config := &BatchQueryConfig{
				MaxConcurrency: concurrency,
				Timeout:        30 * time.Second,
			}

			results := client.GetRecentEventsParallel(ctx, calendarPaths, days, config)

			totalEvents := CountObjectsInResults(results)
			duration := time.Since(start)

			t.Logf("Concurrency %d: %d events in %v", concurrency, totalEvents, duration)
		}
	})

	t.Run("Error_Handling", func(t *testing.T) {
		invalidPaths := append(calendarPaths, "/invalid/calendar/path")

		config := &BatchQueryConfig{
			MaxConcurrency: 3,
			Timeout:        10 * time.Second,
		}

		results := client.GetRecentEventsParallel(ctx, invalidPaths, 7, config)

		successful := FilterSuccessfulResults(results)
		failed := FilterFailedResults(results)

		t.Logf("Successful queries: %d/%d", len(successful), len(results))
		t.Logf("Failed queries: %d/%d", len(failed), len(results))

		if len(failed) == 0 {
			t.Error("Expected at least one failed query for invalid path")
		}

		allObjects, errors := AggregateResults(results)
		t.Logf("Total objects retrieved: %d", len(allObjects))
		t.Logf("Total errors: %d", len(errors))
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		config := &BatchQueryConfig{
			MaxConcurrency: 2,
			Timeout:        5 * time.Second,
		}

		start := time.Now()
		results := client.GetRecentEventsParallel(ctx, calendarPaths, 30, config)
		duration := time.Since(start)

		failed := FilterFailedResults(results)

		t.Logf("Query cancelled after %v", duration)
		t.Logf("Failed queries due to cancellation: %d/%d", len(failed), len(results))

		if duration > 500*time.Millisecond {
			t.Errorf("Expected quick cancellation, took %v", duration)
		}
	})

	t.Run("Large_Time_Range", func(t *testing.T) {
		start := time.Now().AddDate(-1, 0, 0)
		end := time.Now().AddDate(1, 0, 0)

		config := &BatchQueryConfig{
			MaxConcurrency: 5,
			Timeout:        60 * time.Second,
		}

		startTime := time.Now()
		results := client.GetEventsByTimeRangeParallel(ctx, calendarPaths, start, end, config)
		duration := time.Since(startTime)

		totalEvents := CountObjectsInResults(results)
		failed := FilterFailedResults(results)

		t.Logf("Large range query: %d events in %v", totalEvents, duration)
		t.Logf("Failed calendars: %d/%d", len(failed), len(calendars))

		if totalEvents == 0 && len(failed) == 0 {
			t.Log("Warning: No events found in 2-year range")
		}
	})
}

func TestCustomQueryParallelIntegration(t *testing.T) {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")
	if username == "" || password == "" {
		t.Skip("ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables not set")
	}

	client := NewClientWithOptions(username, password)

	ctx := context.Background()

	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		t.Fatalf("Failed to find current user principal: %v", err)
	}

	calendarHomeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		t.Fatalf("Failed to find calendar home set: %v", err)
	}

	calendars, err := client.FindCalendars(ctx, calendarHomeSet)
	if err != nil {
		t.Fatalf("Failed to find calendars: %v", err)
	}

	if len(calendars) < 2 {
		t.Skip("Need at least 2 calendars for this test")
	}

	requests := make([]BatchQueryRequest, 0)

	requests = append(requests, BatchQueryRequest{
		CalendarPath: calendars[0].Href,
		Query: CalendarQuery{
			Properties: []string{"getetag", "calendar-data"},
			TimeRange: &TimeRange{
				Start: time.Now().AddDate(0, 0, -7),
				End:   time.Now(),
			},
		},
	})

	requests = append(requests, BatchQueryRequest{
		CalendarPath: calendars[1].Href,
		Query: CalendarQuery{
			Properties: []string{"getetag", "calendar-data"},
			TimeRange: &TimeRange{
				Start: time.Now(),
				End:   time.Now().AddDate(0, 0, 7),
			},
		},
	})

	config := &BatchQueryConfig{
		MaxConcurrency: 2,
		Timeout:        30 * time.Second,
	}

	results := client.QueryCalendarsParallel(ctx, requests, config)

	for i, result := range results {
		if result.Error != nil {
			t.Logf("Query %d failed: %v", i, result.Error)
		} else {
			t.Logf("Query %d: Retrieved %d objects from %s", i, len(result.Objects), result.CalendarPath)
		}
	}
}
