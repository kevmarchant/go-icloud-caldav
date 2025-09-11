package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
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
