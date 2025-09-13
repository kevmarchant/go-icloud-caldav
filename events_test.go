package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestQueryCalendar(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/event1.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"abc123"</D:getetag>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-uid-123
SUMMARY:Team Meeting
DTSTART:20250115T140000Z
DTEND:20250115T150000Z
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("expected method REPORT, got %s", r.Method)
		}
		if r.URL.Path != "/calendars/test/" {
			t.Errorf("expected path /calendars/test/, got %s", r.URL.Path)
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		Filter: Filter{
			Component: "VEVENT",
		},
	}

	objects, err := client.QueryCalendar(context.Background(), "/calendars/test/", query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	obj := objects[0]
	if obj.UID != "test-uid-123" {
		t.Errorf("expected UID 'test-uid-123', got %s", obj.UID)
	}
	if obj.Summary != "Team Meeting" {
		t.Errorf("expected summary 'Team Meeting', got %s", obj.Summary)
	}
}

func TestGetRecentEvents(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/event1.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"abc123"</D:getetag>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:recent-event
SUMMARY:Recent Event
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("expected method REPORT, got %s", r.Method)
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	objects, err := client.GetRecentEvents(context.Background(), "/calendars/test/", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if objects[0].Summary != "Recent Event" {
		t.Errorf("expected summary 'Recent Event', got %s", objects[0].Summary)
	}
}

func TestGetEventByUID(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/specific.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"specific123"</D:getetag>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:specific-uid
SUMMARY:Specific Event
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	event, err := client.GetEventByUID(context.Background(), "/calendars/test/", "specific-uid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.UID != "specific-uid" {
		t.Errorf("expected UID 'specific-uid', got %s", event.UID)
	}
	if event.Summary != "Specific Event" {
		t.Errorf("expected summary 'Specific Event', got %s", event.Summary)
	}
}

func TestGetEventByUIDNotFound(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	_, err := client.GetEventByUID(context.Background(), "/calendars/test/", "nonexistent-uid")
	if err == nil {
		t.Error("expected error for nonexistent UID, got nil")
	}
}

func TestCountEvents(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/event1.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"tag1"</D:getetag>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/calendars/test/event2.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"tag2"</D:getetag>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/calendars/test/event3.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"tag3"</D:getetag>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	count, err := client.CountEvents(context.Background(), "/calendars/test/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestSearchEvents(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/meeting.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"meeting123"</D:getetag>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:meeting-uid
SUMMARY:Team Meeting
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	events, err := client.SearchEvents(context.Background(), "/calendars/test/", "meeting")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Team Meeting" {
		t.Errorf("expected summary 'Team Meeting', got %s", events[0].Summary)
	}
}

func TestGetEventsByTimeRange(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/ranged.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"ranged123"</D:getetag>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:ranged-uid
SUMMARY:Ranged Event
DTSTART:20250115T100000Z
DTEND:20250115T110000Z
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)

	events, err := client.GetEventsByTimeRange(context.Background(), "/calendars/test/", start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Ranged Event" {
		t.Errorf("expected summary 'Ranged Event', got %s", events[0].Summary)
	}
}
func TestGetAllEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("Expected REPORT method, got %s", r.Method)
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <response>
    <href>/calendars/home/default/event1.ics</href>
    <propstat>
      <status>HTTP/1.1 200 OK</status>
      <prop>
        <getetag>"etag1"</getetag>
        <C:calendar-data>BEGIN:VEVENT
UID:test-event-1
SUMMARY:Test Event 1
DTSTART:20240101T120000Z
DTEND:20240101T130000Z
END:VEVENT</C:calendar-data>
      </prop>
    </propstat>
  </response>
  <response>
    <href>/calendars/home/default/event2.ics</href>
    <propstat>
      <status>HTTP/1.1 200 OK</status>
      <prop>
        <getetag>"etag2"</getetag>
        <C:calendar-data>BEGIN:VEVENT
UID:test-event-2
SUMMARY:Test Event 2
DTSTART:20240201T120000Z
DTEND:20240201T130000Z
END:VEVENT</C:calendar-data>
      </prop>
    </propstat>
  </response>
</multistatus>`))
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.baseURL = server.URL
	events, err := client.GetAllEvents(context.Background(), "/calendars/home/default/")

	if err != nil {
		t.Fatalf("GetAllEvents failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	if events[0].UID != "test-event-1" {
		t.Errorf("Expected UID test-event-1, got %s", events[0].UID)
	}

	if events[1].UID != "test-event-2" {
		t.Errorf("Expected UID test-event-2, got %s", events[1].UID)
	}
}

func TestGetUpcomingEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("Expected REPORT method, got %s", r.Method)
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <response>
    <href>/calendars/home/default/event1.ics</href>
    <propstat>
      <status>HTTP/1.1 200 OK</status>
      <prop>
        <getetag>"etag1"</getetag>
        <C:calendar-data>BEGIN:VEVENT
UID:upcoming-1
SUMMARY:Upcoming Event 1
DTSTART:` + time.Now().Add(24*time.Hour).UTC().Format("20060102T150405Z") + `
DTEND:` + time.Now().Add(25*time.Hour).UTC().Format("20060102T150405Z") + `
END:VEVENT</C:calendar-data>
      </prop>
    </propstat>
  </response>
  <response>
    <href>/calendars/home/default/event2.ics</href>
    <propstat>
      <status>HTTP/1.1 200 OK</status>
      <prop>
        <getetag>"etag2"</getetag>
        <C:calendar-data>BEGIN:VEVENT
UID:upcoming-2
SUMMARY:Upcoming Event 2
DTSTART:` + time.Now().Add(48*time.Hour).UTC().Format("20060102T150405Z") + `
DTEND:` + time.Now().Add(49*time.Hour).UTC().Format("20060102T150405Z") + `
END:VEVENT</C:calendar-data>
      </prop>
    </propstat>
  </response>
  <response>
    <href>/calendars/home/default/event3.ics</href>
    <propstat>
      <status>HTTP/1.1 200 OK</status>
      <prop>
        <getetag>"etag3"</getetag>
        <C:calendar-data>BEGIN:VEVENT
UID:upcoming-3
SUMMARY:Upcoming Event 3
DTSTART:` + time.Now().Add(72*time.Hour).UTC().Format("20060102T150405Z") + `
DTEND:` + time.Now().Add(73*time.Hour).UTC().Format("20060102T150405Z") + `
END:VEVENT</C:calendar-data>
      </prop>
    </propstat>
  </response>
</multistatus>`))
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.baseURL = server.URL

	t.Run("no limit", func(t *testing.T) {
		events, err := client.GetUpcomingEvents(context.Background(), "/calendars/home/default/", 0)

		if err != nil {
			t.Fatalf("GetUpcomingEvents failed: %v", err)
		}

		if len(events) != 3 {
			t.Errorf("Expected 3 events, got %d", len(events))
		}
	})

	t.Run("with limit", func(t *testing.T) {
		events, err := client.GetUpcomingEvents(context.Background(), "/calendars/home/default/", 2)

		if err != nil {
			t.Fatalf("GetUpcomingEvents failed: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("Expected 2 events (limited), got %d", len(events))
		}

		if events[0].UID != "upcoming-1" {
			t.Errorf("Expected first event UID upcoming-1, got %s", events[0].UID)
		}

		if events[1].UID != "upcoming-2" {
			t.Errorf("Expected second event UID upcoming-2, got %s", events[1].UID)
		}
	})
}

func TestClientSetTimeout(t *testing.T) {
	client := NewClient("user", "pass")

	newTimeout := 45 * time.Second
	client.SetTimeout(newTimeout)

	if client.httpClient.Timeout != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, client.httpClient.Timeout)
	}
}

func TestTextCollationConstants(t *testing.T) {
	if CollationASCIICaseMap != "i;ascii-casemap" {
		t.Errorf("expected CollationASCIICaseMap to be 'i;ascii-casemap', got %s", CollationASCIICaseMap)
	}
	if CollationOctet != "i;octet" {
		t.Errorf("expected CollationOctet to be 'i;octet', got %s", CollationOctet)
	}
	if CollationUnicode != "i;unicode-casemap" {
		t.Errorf("expected CollationUnicode to be 'i;unicode-casemap', got %s", CollationUnicode)
	}
}

func TestLogicalOperatorConstants(t *testing.T) {
	if OperatorAND != "AND" {
		t.Errorf("expected OperatorAND to be 'AND', got %s", OperatorAND)
	}
	if OperatorOR != "OR" {
		t.Errorf("expected OperatorOR to be 'OR', got %s", OperatorOR)
	}
	if OperatorNOT != "NOT" {
		t.Errorf("expected OperatorNOT to be 'NOT', got %s", OperatorNOT)
	}
}

func TestQueryWithTextCollation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("expected REPORT request, got %s", r.Method)
		}

		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "collation=") {
			t.Error("expected collation attribute in request")
		}

		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/event1.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"12345"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Test Event with Collation
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	query := AdvancedCalendarQuery{
		Properties: []string{"calendar-data", "getetag"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name: "SUMMARY",
							TextMatch: &TextMatch{
								Value: "Test",
							},
						},
					},
				},
			},
		},
		TextCollation: CollationASCIICaseMap,
	}

	objects, err := client.QueryWithTextCollation(context.Background(), "/calendars/test/", query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "Test Event with Collation") {
		t.Error("expected event data to contain 'Test Event with Collation'")
	}
}

func TestQueryByAttendeeStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/meeting.ics</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:meeting-1
DTSTART:20240101T140000Z
DTEND:20240101T150000Z
SUMMARY:Team Meeting
ORGANIZER:mailto:organizer@example.com
ATTENDEE;PARTSTAT=ACCEPTED:mailto:user@example.com
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	objects, err := client.QueryByAttendeeStatus(context.Background(), "/calendars/test/", "user@example.com", "ACCEPTED")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "PARTSTAT=ACCEPTED") {
		t.Error("expected event to have PARTSTAT=ACCEPTED")
	}
}

func TestQueryWithComplexFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/complex.ics</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:complex-1
DTSTART:20240101T160000Z
DTEND:20240101T170000Z
SUMMARY:Complex Event
LOCATION:Conference Room A
CATEGORIES:MEETING,IMPORTANT
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	filter := ComplexFilter{
		Operator: OperatorAND,
		Conditions: []FilterCondition{
			{
				PropertyName:  "SUMMARY",
				PropertyValue: "Complex",
				Collation:     CollationASCIICaseMap,
			},
			{
				PropertyName:  "LOCATION",
				PropertyValue: "Conference",
				Collation:     CollationASCIICaseMap,
			},
		},
	}

	objects, err := client.QueryWithComplexFilter(context.Background(), "/calendars/test/", filter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "Complex Event") {
		t.Error("expected event data to contain 'Complex Event'")
	}
}

func TestFindEventsWithParameterMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/param.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"param-etag"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:param-1
DTSTART:20240101T180000Z
DTEND:20240101T190000Z
SUMMARY:Parameter Match Event
ATTENDEE;ROLE=REQ-PARTICIPANT;PARTSTAT=ACCEPTED:mailto:user@example.com
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	matches := []PropertyParameterMatch{
		{
			PropertyName:   "ATTENDEE",
			ParameterValue: "mailto:user@example.com",
		},
	}

	objects, err := client.FindEventsWithParameterMatch(context.Background(), "/calendars/test/", matches)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if objects[0].ETag != "\"param-etag\"" {
		t.Errorf("expected ETag '\"param-etag\"', got %s", objects[0].ETag)
	}
}

func TestSearchEventsByText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "<C:prop-filter") {
			t.Error("expected prop-filter in request")
		}

		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/search1.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"search1"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:search-1
DTSTART:20240101T200000Z
DTEND:20240101T210000Z
SUMMARY:Birthday Party
DESCRIPTION:Join us for cake
LOCATION:Home
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	objects, err := client.SearchEventsByText(context.Background(), "/calendars/test/", "Birthday", CollationASCIICaseMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "Birthday Party") {
		t.Error("expected event to contain 'Birthday Party'")
	}
}

func TestQueryByOrganizer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/organizer.ics</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:organizer-1
DTSTART:20240102T100000Z
DTEND:20240102T110000Z
SUMMARY:Staff Meeting
ORGANIZER:mailto:boss@example.com
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	objects, err := client.QueryByOrganizer(context.Background(), "/calendars/test/", "boss@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "ORGANIZER:mailto:boss@example.com") {
		t.Error("expected event to have correct organizer")
	}
}

func TestQueryByCategory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/category1.ics</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:category-1
DTSTART:20240102T120000Z
DTEND:20240102T130000Z
SUMMARY:Project Review
CATEGORIES:WORK,IMPORTANT
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	objects, err := client.QueryByCategory(context.Background(), "/calendars/test/", []string{"WORK", "PERSONAL"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "CATEGORIES:WORK,IMPORTANT") {
		t.Error("expected event to have categories")
	}
}

func TestQueryByPriority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/todo1.ics</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VTODO
UID:todo-1
DTSTART:20240102T140000Z
SUMMARY:High Priority Task
PRIORITY:1
END:VTODO
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	objects, err := client.QueryByPriority(context.Background(), "/calendars/test/", 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "PRIORITY:1") {
		t.Error("expected task to have priority 1")
	}
}

func TestQueryRecurringEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/recurring.ics</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:recurring-1
DTSTART:20240101T090000Z
DTEND:20240101T100000Z
SUMMARY:Daily Standup
RRULE:FREQ=DAILY;BYDAY=MO,TU,WE,TH,FR
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	objects, err := client.QueryRecurringEvents(context.Background(), "/calendars/test/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "RRULE:FREQ=DAILY") {
		t.Error("expected event to have RRULE")
	}
}

func TestQueryByTimeRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "time-range") {
			t.Error("expected time-range in request")
		}

		response := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/timerange.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"timerange-etag"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:timerange-1
DTSTART;TZID=America/New_York:20240102T090000
DTEND;TZID=America/New_York:20240102T100000
SUMMARY:New York Meeting
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := &CalDAVClient{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		logger:     &noopLogger{},
	}

	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	objects, err := client.QueryByTimeRange(context.Background(), "/calendars/test/", start, end, "America/New_York")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}

	if !strings.Contains(objects[0].CalendarData, "TZID=America/New_York") {
		t.Error("expected event to have New York timezone")
	}
}

func TestBuildAdvancedQueryXML(t *testing.T) {
	query := AdvancedCalendarQuery{
		Properties: []string{"calendar-data", "getetag"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name: "SUMMARY",
							TextMatch: &TextMatch{
								Value:     "Test",
								Collation: "i;ascii-casemap",
							},
						},
					},
				},
			},
		},
		TextCollation: CollationUnicode,
		ParameterFilters: []ParameterFilter{
			{
				ParameterName:  "PARTSTAT",
				ParameterValue: "ACCEPTED",
				Collation:      CollationASCIICaseMap,
			},
		},
	}

	xml, err := buildAdvancedQueryXML(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xml, "calendar-query") {
		t.Error("expected calendar-query in XML")
	}
	if !strings.Contains(xml, "comp-filter") {
		t.Error("expected comp-filter in XML")
	}
	if !strings.Contains(xml, "prop-filter") {
		t.Error("expected prop-filter in XML")
	}
	if !strings.Contains(xml, "text-match") {
		t.Error("expected text-match in XML")
	}
	if !strings.Contains(xml, "collation=") {
		t.Error("expected collation attribute in XML")
	}
}

func TestFilterByTimezone(t *testing.T) {
	objects := []CalendarObject{
		{
			CalendarData: "BEGIN:VEVENT\nDTSTART;TZID=America/New_York:20240101T090000\nEND:VEVENT",
		},
		{
			CalendarData: "BEGIN:VEVENT\nDTSTART;TZID=Europe/London:20240101T140000\nEND:VEVENT",
		},
		{
			CalendarData: "BEGIN:VEVENT\nDTSTART:20240101T120000Z\nEND:VEVENT",
		},
	}

	filtered := filterByTimezone(objects, "America/New_York")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered object, got %d", len(filtered))
	}

	if !strings.Contains(filtered[0].CalendarData, "America/New_York") {
		t.Error("expected filtered object to have New York timezone")
	}
}

func TestParameterFilter(t *testing.T) {
	filter := ParameterFilter{
		ParameterName:  "ROLE",
		ParameterValue: "REQ-PARTICIPANT",
		TextMatch:      "user@example.com",
		Collation:      CollationASCIICaseMap,
		NegateMatch:    false,
	}

	if filter.ParameterName != "ROLE" {
		t.Errorf("expected ParameterName to be 'ROLE', got %s", filter.ParameterName)
	}
	if filter.Collation != CollationASCIICaseMap {
		t.Errorf("expected Collation to be ASCII case map, got %s", filter.Collation)
	}
}

func TestComplexFilterCreation(t *testing.T) {
	filter := ComplexFilter{
		Operator: OperatorAND,
		Conditions: []FilterCondition{
			{
				PropertyName:  "SUMMARY",
				PropertyValue: "Meeting",
				Collation:     CollationASCIICaseMap,
			},
			{
				PropertyName:  "LOCATION",
				PropertyValue: "Office",
				Collation:     CollationASCIICaseMap,
				Negate:        false,
			},
		},
	}

	if filter.Operator != OperatorAND {
		t.Errorf("expected Operator to be AND, got %s", filter.Operator)
	}
	if len(filter.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(filter.Conditions))
	}
	if filter.Conditions[0].PropertyName != "SUMMARY" {
		t.Errorf("expected first condition property to be SUMMARY, got %s", filter.Conditions[0].PropertyName)
	}
}

func TestBuildComplexQuery(t *testing.T) {
	filter := ComplexFilter{
		Operator: OperatorOR,
		Conditions: []FilterCondition{
			{
				PropertyName:  "CATEGORIES",
				PropertyValue: "WORK",
				Collation:     CollationUnicode,
			},
			{
				PropertyName:  "PRIORITY",
				PropertyValue: "1",
			},
		},
	}

	query := buildComplexQuery(filter)

	if query.LogicalOperator != OperatorOR {
		t.Errorf("expected LogicalOperator to be OR, got %s", query.LogicalOperator)
	}

	if len(query.Filter.CompFilters) != 1 {
		t.Fatalf("expected 1 comp filter, got %d", len(query.Filter.CompFilters))
	}

	eventFilter := query.Filter.CompFilters[0]
	if len(eventFilter.Props) != 2 {
		t.Errorf("expected 2 prop filters, got %d", len(eventFilter.Props))
	}
}
