package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateRecurringEvent(t *testing.T) {
	tests := []struct {
		name          string
		event         *CalendarObject
		rrule         string
		expectError   bool
		expectedError string
	}{
		{
			name: "daily_recurring",
			event: &CalendarObject{
				Summary:   "Daily Meeting",
				StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)),
			},
			rrule:       "FREQ=DAILY;COUNT=10",
			expectError: false,
		},
		{
			name: "weekly_recurring",
			event: &CalendarObject{
				Summary:   "Weekly Status",
				StartTime: timePtr(time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC)),
			},
			rrule:       "FREQ=WEEKLY;BYDAY=MO,WE,FR",
			expectError: false,
		},
		{
			name: "monthly_recurring",
			event: &CalendarObject{
				Summary:   "Monthly Review",
				StartTime: timePtr(time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			rrule:       "FREQ=MONTHLY;BYMONTHDAY=15",
			expectError: false,
		},
		{
			name: "yearly_recurring",
			event: &CalendarObject{
				Summary:   "Annual Meeting",
				StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
				EndTime:   timePtr(time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)),
			},
			rrule:       "FREQ=YEARLY;COUNT=5",
			expectError: false,
		},
		{
			name: "missing_rrule",
			event: &CalendarObject{
				Summary:   "Event",
				StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
			},
			rrule:         "",
			expectError:   true,
			expectedError: "RRULE is required",
		},
		{
			name: "invalid_rrule",
			event: &CalendarObject{
				Summary:   "Event",
				StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
			},
			rrule:         "INVALID",
			expectError:   true,
			expectedError: "RRULE must specify FREQ",
		},
		{
			name: "missing_freq",
			event: &CalendarObject{
				Summary:   "Event",
				StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
			},
			rrule:         "COUNT=10",
			expectError:   true,
			expectedError: "RRULE must specify FREQ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}))
			defer server.Close()

			client := &CalDAVClient{
				baseURL:    server.URL,
				authHeader: "Basic dGVzdDp0ZXN0",
				httpClient: &http.Client{},
				logger:     &testLogger{},
			}

			err := client.CreateRecurringEvent("/calendars/test/", tt.event, tt.rrule)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.event.RecurrenceRule != tt.rrule {
					t.Errorf("Expected RecurrenceRule to be %q, got %q", tt.rrule, tt.event.RecurrenceRule)
				}
			}
		})
	}
}

func TestUpdateRecurrencePattern(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	event := &CalendarObject{
		UID:            "test-event",
		Summary:        "Recurring Event",
		StartTime:      timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
		EndTime:        timePtr(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)),
		RecurrenceRule: "FREQ=DAILY;COUNT=10",
	}

	newRRule := "FREQ=WEEKLY;BYDAY=MO,WE,FR"
	err := client.UpdateRecurrencePattern("/calendars/test/", event, newRRule, "etag123")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if event.RecurrenceRule != newRRule {
		t.Errorf("Expected RecurrenceRule to be %q, got %q", newRRule, event.RecurrenceRule)
	}

	if event.LastModified == nil {
		t.Error("Expected LastModified to be set")
	}
}

func TestDeleteRecurrenceInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	event := &CalendarObject{
		UID:            "test-event",
		Summary:        "Recurring Event",
		StartTime:      timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
		EndTime:        timePtr(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)),
		RecurrenceRule: "FREQ=DAILY;COUNT=10",
	}

	instanceDate := time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC)
	err := client.DeleteRecurrenceInstance("/calendars/test/", event, instanceDate, "etag123")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(event.ExceptionDates) != 1 {
		t.Errorf("Expected 1 exception date, got %d", len(event.ExceptionDates))
	}

	if !event.ExceptionDates[0].Equal(instanceDate) {
		t.Errorf("Expected exception date %v, got %v", instanceDate, event.ExceptionDates[0])
	}
}

func TestDeleteRecurrenceInstance_NonRecurring(t *testing.T) {
	client := &CalDAVClient{
		baseURL:    "http://test.com",
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	event := &CalendarObject{
		UID:       "test-event",
		Summary:   "Non-recurring Event",
		StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
	}

	instanceDate := time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC)
	err := client.DeleteRecurrenceInstance("/calendars/test/", event, instanceDate, "etag123")

	if err == nil {
		t.Error("Expected error for non-recurring event")
	}

	if !strings.Contains(err.Error(), "not recurring") {
		t.Errorf("Expected 'not recurring' error, got %v", err)
	}
}

func TestDeleteRecurrenceInstance_AlreadyDeleted(t *testing.T) {
	client := &CalDAVClient{
		baseURL:    "http://test.com",
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	instanceDate := time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC)
	event := &CalendarObject{
		UID:            "test-event",
		Summary:        "Recurring Event",
		StartTime:      timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
		RecurrenceRule: "FREQ=DAILY",
		ExceptionDates: []time.Time{instanceDate},
	}

	err := client.DeleteRecurrenceInstance("/calendars/test/", event, instanceDate, "etag123")

	if err == nil {
		t.Error("Expected error for already deleted instance")
	}

	if !strings.Contains(err.Error(), "already deleted") {
		t.Errorf("Expected 'already deleted' error, got %v", err)
	}
}

func TestExpandRecurringEvent(t *testing.T) {
	client := &CalDAVClient{
		logger: &testLogger{},
	}

	event := &CalendarObject{
		UID:            "test-event",
		Summary:        "Daily Event",
		StartTime:      timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
		EndTime:        timePtr(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)),
		RecurrenceRule: "FREQ=DAILY;COUNT=5",
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	occurrences, err := client.ExpandRecurringEvent(event, start, end)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(occurrences) != 5 {
		t.Errorf("Expected 5 occurrences, got %d", len(occurrences))
	}

	for i, occ := range occurrences {
		expectedStart := time.Date(2024, 1, i+1, 10, 0, 0, 0, time.UTC)
		if !occ.StartTime.Equal(expectedStart) {
			t.Errorf("Occurrence %d: expected start %v, got %v", i, expectedStart, occ.StartTime)
		}

		expectedEnd := time.Date(2024, 1, i+1, 11, 0, 0, 0, time.UTC)
		if !occ.EndTime.Equal(expectedEnd) {
			t.Errorf("Occurrence %d: expected end %v, got %v", i, expectedEnd, occ.EndTime)
		}

		if occ.UID != event.UID {
			t.Errorf("Occurrence %d: expected UID %q, got %q", i, event.UID, occ.UID)
		}
	}
}

func TestExpandRecurringEvent_WithExceptions(t *testing.T) {
	client := &CalDAVClient{
		logger: &testLogger{},
	}

	event := &CalendarObject{
		UID:            "test-event",
		Summary:        "Daily Event",
		StartTime:      timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
		EndTime:        timePtr(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)),
		RecurrenceRule: "FREQ=DAILY;COUNT=5",
		ExceptionDates: []time.Time{
			time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC),
		},
	}

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	occurrences, err := client.ExpandRecurringEvent(event, start, end)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(occurrences) != 4 {
		t.Errorf("Expected 4 occurrences (5 - 1 exception), got %d", len(occurrences))
	}

	for _, occ := range occurrences {
		if occ.StartTime.Equal(time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC)) {
			t.Error("Exception date should not be in occurrences")
		}
	}
}

func TestGetRecurringEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <response>
    <href>/calendars/test/event1.ics</href>
    <propstat>
      <prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-recurring-event
SUMMARY:Weekly Meeting
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
RRULE:FREQ=WEEKLY;BYDAY=MO
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`))
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	events, err := client.GetRecurringEvents("/calendars/test/")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if events == nil {
		t.Error("Expected events, got nil")
	} else if len(events) == 0 {
		t.Error("Expected at least one event, got empty slice")
	}
}

func TestValidateRRule(t *testing.T) {
	tests := []struct {
		name          string
		rrule         string
		expectError   bool
		expectedError string
	}{
		{
			name:        "valid_daily",
			rrule:       "FREQ=DAILY;COUNT=10",
			expectError: false,
		},
		{
			name:        "valid_weekly",
			rrule:       "FREQ=WEEKLY;BYDAY=MO,WE,FR",
			expectError: false,
		},
		{
			name:        "valid_monthly",
			rrule:       "FREQ=MONTHLY;BYMONTHDAY=15",
			expectError: false,
		},
		{
			name:        "valid_yearly",
			rrule:       "FREQ=YEARLY;COUNT=5",
			expectError: false,
		},
		{
			name:          "empty_rrule",
			rrule:         "",
			expectError:   true,
			expectedError: "cannot be empty",
		},
		{
			name:          "missing_freq",
			rrule:         "COUNT=10",
			expectError:   true,
			expectedError: "must specify FREQ",
		},
		{
			name:          "invalid_freq",
			rrule:         "FREQ=INVALID",
			expectError:   true,
			expectedError: "invalid FREQ value",
		},
		{
			name:          "both_count_and_until",
			rrule:         "FREQ=DAILY;COUNT=10;UNTIL=20241231T000000Z",
			expectError:   true,
			expectedError: "COUNT and UNTIL cannot both be specified",
		},
		{
			name:        "valid_with_until",
			rrule:       "FREQ=DAILY;UNTIL=20241231T000000Z",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRRule(tt.rrule)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedError != "" && !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBuildRRule(t *testing.T) {
	tests := []struct {
		name     string
		freq     string
		interval int
		count    int
		until    *time.Time
		byDay    []string
		expected string
	}{
		{
			name:     "simple_daily",
			freq:     "daily",
			interval: 1,
			count:    10,
			expected: "FREQ=DAILY;COUNT=10",
		},
		{
			name:     "weekly_with_interval",
			freq:     "weekly",
			interval: 2,
			count:    0,
			expected: "FREQ=WEEKLY;INTERVAL=2",
		},
		{
			name:     "weekly_with_byday",
			freq:     "weekly",
			interval: 1,
			byDay:    []string{"MO", "WE", "FR"},
			expected: "FREQ=WEEKLY;BYDAY=MO,WE,FR",
		},
		{
			name:     "monthly_with_count",
			freq:     "monthly",
			interval: 1,
			count:    12,
			expected: "FREQ=MONTHLY;COUNT=12",
		},
		{
			name:     "yearly_with_until",
			freq:     "yearly",
			interval: 1,
			until:    timePtr(time.Date(2030, 12, 31, 0, 0, 0, 0, time.UTC)),
			expected: "FREQ=YEARLY;UNTIL=20301231T000000Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildRRule(tt.freq, tt.interval, tt.count, tt.until, tt.byDay)
			if result != tt.expected {
				t.Errorf("BuildRRule() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCreateRecurringEventWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := &CalendarObject{
		Summary:   "Test Event",
		StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
	}

	err := client.CreateRecurringEventWithContext(ctx, "/calendars/test/", event, "FREQ=DAILY")

	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestCreateRecurringEventWithExceptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	event := &CalendarObject{
		Summary:   "Recurring with Exceptions",
		StartTime: timePtr(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)),
		EndTime:   timePtr(time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)),
	}

	exceptions := []time.Time{
		time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
	}

	err := client.CreateRecurringEventWithExceptions("/calendars/test/", event, "FREQ=DAILY;COUNT=15", exceptions)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if event.RecurrenceRule != "FREQ=DAILY;COUNT=15" {
		t.Errorf("Expected RecurrenceRule to be set, got %q", event.RecurrenceRule)
	}

	if len(event.ExceptionDates) != 2 {
		t.Errorf("Expected 2 exception dates, got %d", len(event.ExceptionDates))
	}
}
