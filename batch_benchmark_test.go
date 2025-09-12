package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkQueryCalendarsSerial(b *testing.B) {
	benchmarkConfigs := []struct {
		name         string
		numCalendars int
		delay        time.Duration
	}{
		{"5calendars_NoDelay", 5, 0},
		{"5calendars_10msDelay", 5, 10 * time.Millisecond},
		{"10calendars_NoDelay", 10, 0},
		{"10calendars_10msDelay", 10, 10 * time.Millisecond},
		{"20calendars_NoDelay", 20, 0},
		{"20calendars_5msDelay", 20, 5 * time.Millisecond},
	}

	for _, bc := range benchmarkConfigs {
		// Capture loop variable for closure
		bc := bc
		b.Run(bc.name, func(b *testing.B) {
			server := createBenchmarkServer(bc.delay)
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.baseURL = server.URL

			// Use a custom HTTP client with connection pooling settings
			transport := &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			}
			client.httpClient = &http.Client{
				Transport: transport,
				Timeout:   30 * time.Second,
			}

			calendars := make([]string, bc.numCalendars)
			for i := 0; i < bc.numCalendars; i++ {
				calendars[i] = fmt.Sprintf("/calendar%d", i)
			}

			query := CalendarQuery{
				Properties: []string{"getetag", "calendar-data"},
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, calPath := range calendars {
					_, err := client.QueryCalendar(ctx, calPath, query)
					if err != nil {
						b.Fatalf("query failed: %v", err)
					}
				}
			}
		})
	}
}

func BenchmarkQueryCalendarsParallel(b *testing.B) {
	benchmarkConfigs := []struct {
		name           string
		numCalendars   int
		maxConcurrency int
		delay          time.Duration
	}{
		{"5calendars_2workers_NoDelay", 5, 2, 0},
		{"5calendars_5workers_NoDelay", 5, 5, 0},
		{"5calendars_2workers_10msDelay", 5, 2, 10 * time.Millisecond},
		{"5calendars_5workers_10msDelay", 5, 5, 10 * time.Millisecond},
		{"10calendars_3workers_NoDelay", 10, 3, 0},
		{"10calendars_5workers_NoDelay", 10, 5, 0},
		{"10calendars_10workers_NoDelay", 10, 10, 0},
		{"10calendars_5workers_10msDelay", 10, 5, 10 * time.Millisecond},
		{"20calendars_5workers_NoDelay", 20, 5, 0},
		{"20calendars_10workers_NoDelay", 20, 10, 0},
		{"20calendars_5workers_5msDelay", 20, 5, 5 * time.Millisecond},
		{"20calendars_10workers_5msDelay", 20, 10, 5 * time.Millisecond},
	}

	for _, bc := range benchmarkConfigs {
		// Capture loop variable for closure
		bc := bc
		b.Run(bc.name, func(b *testing.B) {
			// Create server once before benchmark starts
			server := createBenchmarkServer(bc.delay)
			defer server.Close()

			// Create client with connection pooling
			client := NewClient("test@example.com", "password")
			client.baseURL = server.URL

			// Use a custom HTTP client with connection pooling settings
			transport := &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			}
			client.httpClient = &http.Client{
				Transport: transport,
				Timeout:   30 * time.Second,
			}

			requests := make([]BatchQueryRequest, bc.numCalendars)
			for i := 0; i < bc.numCalendars; i++ {
				requests[i] = BatchQueryRequest{
					CalendarPath: fmt.Sprintf("/calendar%d", i),
					Query: CalendarQuery{
						Properties: []string{"getetag", "calendar-data"},
					},
				}
			}

			config := &BatchQueryConfig{
				MaxConcurrency: bc.maxConcurrency,
				Timeout:        30 * time.Second,
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results := client.QueryCalendarsParallel(ctx, requests, config)
				for _, result := range results {
					if result.Error != nil {
						b.Fatalf("query failed: %v", result.Error)
					}
				}
			}
		})
	}
}

func BenchmarkAggregateResults(b *testing.B) {
	benchmarkSizes := []int{10, 50, 100, 500, 1000}

	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("%d_results", size), func(b *testing.B) {
			results := make([]BatchQueryResult, size)
			for i := 0; i < size; i++ {
				numObjects := 10
				objects := make([]CalendarObject, numObjects)
				for j := 0; j < numObjects; j++ {
					objects[j] = CalendarObject{
						Href:         fmt.Sprintf("/calendar%d/event%d.ics", i, j),
						ETag:         fmt.Sprintf("etag-%d-%d", i, j),
						CalendarData: fmt.Sprintf("BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:event-%d-%d\nEND:VEVENT\nEND:VCALENDAR", i, j),
					}
				}

				if i%10 == 0 {
					results[i] = BatchQueryResult{
						CalendarPath: fmt.Sprintf("/calendar%d", i),
						Error:        fmt.Errorf("simulated error for calendar %d", i),
					}
				} else {
					results[i] = BatchQueryResult{
						CalendarPath: fmt.Sprintf("/calendar%d", i),
						Objects:      objects,
					}
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				objects, errors := AggregateResults(results)
				_ = objects
				_ = errors
			}
		})
	}
}

func BenchmarkWorkerPoolOverhead(b *testing.B) {
	workerCounts := []int{1, 2, 5, 10, 20, 50}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("%d_workers", workers), func(b *testing.B) {
			server := createBenchmarkServer(0)
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.baseURL = server.URL

			requests := make([]BatchQueryRequest, 100)
			for i := 0; i < 100; i++ {
				requests[i] = BatchQueryRequest{
					CalendarPath: fmt.Sprintf("/calendar%d", i),
					Query: CalendarQuery{
						Properties: []string{"getetag", "calendar-data"},
					},
				}
			}

			config := &BatchQueryConfig{
				MaxConcurrency: workers,
				Timeout:        30 * time.Second,
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results := client.QueryCalendarsParallel(ctx, requests, config)
				if len(results) != 100 {
					b.Fatalf("expected 100 results, got %d", len(results))
				}
			}
		})
	}
}

func createBenchmarkServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)

		calendarPath := r.URL.Path
		response := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>%s/event1.ics</href>
    <propstat>
      <prop>
        <getetag>"etag-1"</getetag>
        <calendar-data xmlns="urn:ietf:params:xml:ns:caldav">BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Benchmark//Test//EN
BEGIN:VEVENT
UID:benchmark-event-1
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Benchmark Event 1
DESCRIPTION:This is a benchmark test event
LOCATION:Test Location
END:VEVENT
END:VCALENDAR</calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
  <response>
    <href>%s/event2.ics</href>
    <propstat>
      <prop>
        <getetag>"etag-2"</getetag>
        <calendar-data xmlns="urn:ietf:params:xml:ns:caldav">BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Benchmark//Test//EN
BEGIN:VEVENT
UID:benchmark-event-2
DTSTART:20240102T140000Z
DTEND:20240102T150000Z
SUMMARY:Benchmark Event 2
DESCRIPTION:Another benchmark test event
LOCATION:Another Location
END:VEVENT
END:VCALENDAR</calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, calendarPath, calendarPath)

		_, _ = fmt.Fprint(w, response)
	}))
}

func BenchmarkSpeedImprovement(b *testing.B) {
	numCalendars := 10
	delay := 20 * time.Millisecond

	server := createBenchmarkServer(delay)
	defer server.Close()

	client := NewClient("test@example.com", "password")
	client.baseURL = server.URL

	// Use a custom HTTP client with connection pooling settings
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	client.httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	b.Run("Serial", func(b *testing.B) {
		calendars := make([]string, numCalendars)
		for i := 0; i < numCalendars; i++ {
			calendars[i] = fmt.Sprintf("/calendar%d", i)
		}

		query := CalendarQuery{
			Properties: []string{"getetag", "calendar-data"},
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			for _, calPath := range calendars {
				_, err := client.QueryCalendar(ctx, calPath, query)
				if err != nil {
					b.Fatalf("query failed: %v", err)
				}
			}
			elapsed := time.Since(start)
			b.Logf("Serial: %d calendars in %v", numCalendars, elapsed)
		}
	})

	b.Run("Parallel_5workers", func(b *testing.B) {
		requests := make([]BatchQueryRequest, numCalendars)
		for i := 0; i < numCalendars; i++ {
			requests[i] = BatchQueryRequest{
				CalendarPath: fmt.Sprintf("/calendar%d", i),
				Query: CalendarQuery{
					Properties: []string{"getetag", "calendar-data"},
				},
			}
		}

		config := &BatchQueryConfig{
			MaxConcurrency: 5,
			Timeout:        30 * time.Second,
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			results := client.QueryCalendarsParallel(ctx, requests, config)
			for _, result := range results {
				if result.Error != nil {
					b.Fatalf("query failed: %v", result.Error)
				}
			}
			elapsed := time.Since(start)
			b.Logf("Parallel (5 workers): %d calendars in %v", numCalendars, elapsed)
		}
	})

	b.Run("Parallel_10workers", func(b *testing.B) {
		requests := make([]BatchQueryRequest, numCalendars)
		for i := 0; i < numCalendars; i++ {
			requests[i] = BatchQueryRequest{
				CalendarPath: fmt.Sprintf("/calendar%d", i),
				Query: CalendarQuery{
					Properties: []string{"getetag", "calendar-data"},
				},
			}
		}

		config := &BatchQueryConfig{
			MaxConcurrency: 10,
			Timeout:        30 * time.Second,
		}

		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			start := time.Now()
			results := client.QueryCalendarsParallel(ctx, requests, config)
			for _, result := range results {
				if result.Error != nil {
					b.Fatalf("query failed: %v", result.Error)
				}
			}
			elapsed := time.Since(start)
			b.Logf("Parallel (10 workers): %d calendars in %v", numCalendars, elapsed)
		}
	})
}
