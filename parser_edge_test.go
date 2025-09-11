package caldav

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestExtractCalendarObjectsFromResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		xmlData   string
		expected  int
		checkFunc func(*testing.T, []CalendarObject)
	}{
		{
			name: "Calendar data without VEVENT",
			xmlData: `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/test.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:test
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`,
			expected: 1,
			checkFunc: func(t *testing.T, objects []CalendarObject) {
				if objects[0].Summary != "" {
					t.Errorf("expected empty summary, got %s", objects[0].Summary)
				}
			},
		},
		{
			name: "Invalid calendar data",
			xmlData: `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/test.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>NOT VALID ICAL DATA</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`,
			expected: 1,
			checkFunc: func(t *testing.T, objects []CalendarObject) {
				if objects[0].CalendarData != "NOT VALID ICAL DATA" {
					t.Errorf("expected raw data preserved, got %s", objects[0].CalendarData)
				}
			},
		},
		{
			name: "Event without dates",
			xmlData: `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/test.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:no-dates-uid
SUMMARY:Event without dates
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`,
			expected: 1,
			checkFunc: func(t *testing.T, objects []CalendarObject) {
				if objects[0].StartTime != nil {
					t.Error("expected nil start time")
				}
				if objects[0].EndTime != nil {
					t.Error("expected nil end time")
				}
				if objects[0].Summary != "Event without dates" {
					t.Errorf("expected summary 'Event without dates', got %s", objects[0].Summary)
				}
			},
		},
		{
			name: "Event with invalid date format",
			xmlData: `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/test.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:bad-dates
SUMMARY:Bad dates
DTSTART:NOT_A_DATE
DTEND:ALSO_NOT_A_DATE
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`,
			expected: 1,
			checkFunc: func(t *testing.T, objects []CalendarObject) {
				if objects[0].StartTime != nil {
					t.Error("expected nil start time for invalid date")
				}
				if objects[0].EndTime != nil {
					t.Error("expected nil end time for invalid date")
				}
			},
		},
		{
			name: "Multiple VEVENTs in one calendar",
			xmlData: `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/test.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:first-event
SUMMARY:First Event
END:VEVENT
BEGIN:VEVENT
UID:second-event
SUMMARY:Second Event
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`,
			expected: 1,
			checkFunc: func(t *testing.T, objects []CalendarObject) {
				if objects[0].UID != "second-event" {
					t.Errorf("expected second event UID (last one wins), got %s", objects[0].UID)
				}
			},
		},
		{
			name: "Event with line folding",
			xmlData: `<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/test.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:folded-uid
SUMMARY:This is a very long summary that would normally be folded in
 a real iCalendar file to meet the 75 character line limit requirement
DESCRIPTION:This is a description with
 multiple lines that are
 folded using the iCalendar
 folding convention
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`,
			expected: 1,
			checkFunc: func(t *testing.T, objects []CalendarObject) {
				expectedSummary := "This is a very long summary that would normally be folded in"
				if objects[0].Summary != expectedSummary {
					t.Errorf("expected summary %q, got %q", expectedSummary, objects[0].Summary)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.xmlData)
			resp, err := parseMultiStatusResponse(reader)
			if err != nil {
				t.Fatalf("unexpected error parsing: %v", err)
			}

			objects := extractCalendarObjectsFromResponse(resp)

			if len(objects) != tt.expected {
				t.Errorf("expected %d objects, got %d", tt.expected, len(objects))
			}

			if tt.checkFunc != nil && len(objects) > 0 {
				tt.checkFunc(t, objects)
			}
		})
	}
}

func TestParseICalTime_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		checkFunc func(*testing.T, *time.Time)
	}{
		{
			name:  "Empty string",
			input: "",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm != nil {
					t.Error("expected nil for empty string")
				}
			},
		},
		{
			name:  "Invalid format",
			input: "DTSTART:2025-01-11",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm != nil {
					t.Error("expected nil for invalid format")
				}
			},
		},
		{
			name:  "Date only format",
			input: "DTSTART:20250111",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm == nil {
					t.Error("expected non-nil time")
				} else {
					expected := time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC)
					if !tm.Equal(expected) {
						t.Errorf("expected %v, got %v", expected, tm)
					}
				}
			},
		},
		{
			name:  "DateTime with Z",
			input: "DTSTART:20250111T123045Z",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm == nil {
					t.Error("expected non-nil time")
				} else {
					expected := time.Date(2025, 1, 11, 12, 30, 45, 0, time.UTC)
					if !tm.Equal(expected) {
						t.Errorf("expected %v, got %v", expected, tm)
					}
				}
			},
		},
		{
			name:  "DateTime without Z",
			input: "DTEND:20250111T123045",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm == nil {
					t.Error("expected non-nil time")
				} else {
					expected := time.Date(2025, 1, 11, 12, 30, 45, 0, time.UTC)
					if !tm.Equal(expected) {
						t.Errorf("expected %v, got %v", expected, tm)
					}
				}
			},
		},
		{
			name:  "Invalid month",
			input: "DTSTART:20251311T120000Z",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm != nil {
					t.Error("expected nil for invalid month")
				}
			},
		},
		{
			name:  "Invalid day",
			input: "DTSTART:20250132T120000Z",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm != nil {
					t.Error("expected nil for invalid day")
				}
			},
		},
		{
			name:  "Leap year date",
			input: "DTSTART:20240229T120000Z",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm == nil {
					t.Error("expected non-nil time")
				} else {
					expected := time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)
					if !tm.Equal(expected) {
						t.Errorf("expected %v, got %v", expected, tm)
					}
				}
			},
		},
		{
			name:  "Non-leap year Feb 29",
			input: "DTSTART:20250229T120000Z",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm != nil {
					t.Error("expected nil for invalid leap year date")
				}
			},
		},
		{
			name:  "Line without colon",
			input: "NOTAVALIDLINE",
			checkFunc: func(t *testing.T, tm *time.Time) {
				if tm != nil {
					t.Error("expected nil for line without colon")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := parseICalTime(tt.input)
			if tt.checkFunc != nil {
				tt.checkFunc(t, tm)
			}
		})
	}
}

func TestReaderErrorHandling(t *testing.T) {
	errReader := &errorReader{err: io.ErrUnexpectedEOF}

	_, err := parseMultiStatusResponse(errReader)
	if err == nil {
		t.Error("expected error from failing reader")
	}
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestHTTPClientTimeout(t *testing.T) {
	client := NewClient("test@example.com", "password")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(2 * time.Millisecond)

	_, err := client.FindCurrentUserPrincipal(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected context deadline exceeded error, got: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	client := NewClient("test@example.com", "password")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.FindCalendars(ctx, "https://caldav.icloud.com/123/calendars/")
	if err == nil {
		t.Error("expected cancellation error")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}
