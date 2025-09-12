package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQueryCalendarsParallel(t *testing.T) {
	tests := []struct {
		name           string
		numCalendars   int
		maxConcurrency int
		expectedErrors int
		serverDelay    time.Duration
	}{
		{
			name:           "single calendar",
			numCalendars:   1,
			maxConcurrency: 1,
			expectedErrors: 0,
			serverDelay:    0,
		},
		{
			name:           "multiple calendars sequential",
			numCalendars:   3,
			maxConcurrency: 1,
			expectedErrors: 0,
			serverDelay:    10 * time.Millisecond,
		},
		{
			name:           "multiple calendars parallel",
			numCalendars:   5,
			maxConcurrency: 3,
			expectedErrors: 0,
			serverDelay:    10 * time.Millisecond,
		},
		{
			name:           "with errors",
			numCalendars:   4,
			maxConcurrency: 2,
			expectedErrors: 2,
			serverDelay:    0,
		},
		{
			name:           "more workers than requests",
			numCalendars:   2,
			maxConcurrency: 5,
			expectedErrors: 0,
			serverDelay:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverDelay > 0 {
					time.Sleep(tt.serverDelay)
				}

				calendarPath := r.URL.Path
				calendarNum := strings.TrimPrefix(calendarPath, "/calendar")

				if tt.expectedErrors > 0 && (calendarNum == "1" || calendarNum == "3") {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>%s/event1.ics</href>
    <propstat>
      <prop>
        <getetag>"12345"</getetag>
        <calendar-data xmlns="urn:ietf:params:xml:ns:caldav">BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-%s-1
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Test Event %s-1
END:VEVENT
END:VCALENDAR</calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, calendarPath, calendarNum, calendarNum)
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.baseURL = server.URL

			requests := make([]BatchQueryRequest, tt.numCalendars)
			for i := 0; i < tt.numCalendars; i++ {
				requests[i] = BatchQueryRequest{
					CalendarPath: fmt.Sprintf("/calendar%d", i),
					Query: CalendarQuery{
						Properties: []string{"getetag", "calendar-data"},
					},
				}
			}

			config := &BatchQueryConfig{
				MaxConcurrency: tt.maxConcurrency,
				Timeout:        5 * time.Second,
			}

			ctx := context.Background()
			results := client.QueryCalendarsParallel(ctx, requests, config)

			if len(results) != tt.numCalendars {
				t.Errorf("expected %d results, got %d", tt.numCalendars, len(results))
			}

			errorCount := 0
			successCount := 0
			for _, result := range results {
				if result.Error != nil {
					errorCount++
				} else {
					successCount++
					if len(result.Objects) == 0 {
						t.Errorf("successful query returned no objects for calendar %s", result.CalendarPath)
					}
				}
			}

			expectedSuccess := tt.numCalendars - tt.expectedErrors
			if successCount != expectedSuccess {
				t.Errorf("expected %d successful queries, got %d", expectedSuccess, successCount)
			}
		})
	}
}

func TestGetRecentEventsParallel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("expected REPORT method, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>%s/event1.ics</href>
    <propstat>
      <prop>
        <getetag>"12345"</getetag>
        <calendar-data xmlns="urn:ietf:params:xml:ns:caldav">BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:recent-event-1
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Recent Event
END:VEVENT
END:VCALENDAR</calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("test@example.com", "password")
	client.baseURL = server.URL

	calendarPaths := []string{"/calendar1", "/calendar2", "/calendar3"}

	ctx := context.Background()
	results := client.GetRecentEventsParallel(ctx, calendarPaths, 7, nil)

	if len(results) != len(calendarPaths) {
		t.Errorf("expected %d results, got %d", len(calendarPaths), len(results))
	}

	for i, result := range results {
		if result.Error != nil {
			t.Errorf("unexpected error for calendar %s: %v", calendarPaths[i], result.Error)
		}
		if len(result.Objects) == 0 {
			t.Errorf("no objects returned for calendar %s", calendarPaths[i])
		}
	}
}

func TestAggregateResults(t *testing.T) {
	results := []BatchQueryResult{
		{
			CalendarPath: "/calendar1",
			Objects: []CalendarObject{
				{Href: "/calendar1/event1.ics", ETag: "etag1"},
				{Href: "/calendar1/event2.ics", ETag: "etag2"},
			},
			Error: nil,
		},
		{
			CalendarPath: "/calendar2",
			Objects:      nil,
			Error:        fmt.Errorf("connection error"),
		},
		{
			CalendarPath: "/calendar3",
			Objects: []CalendarObject{
				{Href: "/calendar3/event1.ics", ETag: "etag3"},
			},
			Error: nil,
		},
	}

	objects, errors := AggregateResults(results)

	if len(objects) != 3 {
		t.Errorf("expected 3 objects, got %d", len(objects))
	}

	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}

	if !strings.Contains(errors[0].Error(), "calendar2") {
		t.Errorf("error should mention calendar2: %v", errors[0])
	}
}

func TestCountObjectsInResults(t *testing.T) {
	results := []BatchQueryResult{
		{
			Objects: []CalendarObject{{}, {}, {}},
			Error:   nil,
		},
		{
			Objects: nil,
			Error:   fmt.Errorf("error"),
		},
		{
			Objects: []CalendarObject{{}, {}},
			Error:   nil,
		},
	}

	count := CountObjectsInResults(results)
	if count != 5 {
		t.Errorf("expected 5 objects, got %d", count)
	}
}

func TestFilterSuccessfulResults(t *testing.T) {
	results := []BatchQueryResult{
		{CalendarPath: "/cal1", Error: nil},
		{CalendarPath: "/cal2", Error: fmt.Errorf("error")},
		{CalendarPath: "/cal3", Error: nil},
	}

	successful := FilterSuccessfulResults(results)
	if len(successful) != 2 {
		t.Errorf("expected 2 successful results, got %d", len(successful))
	}

	for _, result := range successful {
		if result.Error != nil {
			t.Errorf("successful result should not have error: %v", result.Error)
		}
	}
}

func TestFilterFailedResults(t *testing.T) {
	results := []BatchQueryResult{
		{CalendarPath: "/cal1", Error: nil},
		{CalendarPath: "/cal2", Error: fmt.Errorf("error1")},
		{CalendarPath: "/cal3", Error: fmt.Errorf("error2")},
	}

	failed := FilterFailedResults(results)
	if len(failed) != 2 {
		t.Errorf("expected 2 failed results, got %d", len(failed))
	}

	for _, result := range failed {
		if result.Error == nil {
			t.Errorf("failed result should have error")
		}
	}
}

func TestDefaultBatchQueryConfig(t *testing.T) {
	config := DefaultBatchQueryConfig()

	if config.MaxConcurrency != 5 {
		t.Errorf("expected default MaxConcurrency to be 5, got %d", config.MaxConcurrency)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("expected default Timeout to be 30s, got %v", config.Timeout)
	}
}

func TestQueryCalendarsParallelWithNilConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>/event.ics</href>
    <propstat>
      <prop>
        <getetag>"12345"</getetag>
        <calendar-data xmlns="urn:ietf:params:xml:ns:caldav">BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR</calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`)
	}))
	defer server.Close()

	client := NewClient("test@example.com", "password")
	client.baseURL = server.URL

	requests := []BatchQueryRequest{
		{
			CalendarPath: "/calendar1",
			Query:        CalendarQuery{Properties: []string{"getetag", "calendar-data"}},
		},
	}

	ctx := context.Background()
	results := client.QueryCalendarsParallel(ctx, requests, nil)

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
}

func TestQueryCalendarsParallelEmptyRequests(t *testing.T) {
	client := NewClient("test@example.com", "password")

	ctx := context.Background()
	results := client.QueryCalendarsParallel(ctx, []BatchQueryRequest{}, nil)

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty requests, got %d", len(results))
	}
}

func TestQueryCalendarsParallelContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(blocked)

	client := NewClient("test@example.com", "password")
	client.baseURL = server.URL

	requests := []BatchQueryRequest{
		{CalendarPath: "/calendar1", Query: CalendarQuery{}},
		{CalendarPath: "/calendar2", Query: CalendarQuery{}},
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	results := client.QueryCalendarsParallel(ctx, requests, &BatchQueryConfig{
		MaxConcurrency: 2,
		Timeout:        5 * time.Second,
	})

	errorsFound := 0
	for _, result := range results {
		if result.Error != nil {
			errorsFound++
		}
	}

	if errorsFound == 0 {
		t.Error("expected errors due to context cancellation")
	}
}
