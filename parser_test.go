package caldav

import (
	"strings"
	"testing"
	"time"
)

func TestParseMultiStatusResponse(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Test Calendar</D:displayname>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/calendars/test2/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 404 Not Found</D:status>
			<D:prop/>
		</D:propstat>
	</D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Responses) != 2 {
		t.Errorf("expected 2 responses, got %d", len(resp.Responses))
	}

	firstResp := resp.Responses[0]
	if firstResp.Href != "/calendars/test/" {
		t.Errorf("expected href /calendars/test/, got %s", firstResp.Href)
	}

	if len(firstResp.Propstat) != 1 {
		t.Errorf("expected 1 propstat, got %d", len(firstResp.Propstat))
	}

	ps := firstResp.Propstat[0]
	if ps.Status != 200 {
		t.Errorf("expected status 200, got %d", ps.Status)
	}

	if ps.Prop.DisplayName != "Test Calendar" {
		t.Errorf("expected display name 'Test Calendar', got %s", ps.Prop.DisplayName)
	}

	if len(ps.Prop.ResourceType) != 2 {
		t.Errorf("expected 2 resource types, got %d", len(ps.Prop.ResourceType))
	}

	secondResp := resp.Responses[1]
	if secondResp.Propstat[0].Status != 404 {
		t.Errorf("expected status 404 for second response, got %d", secondResp.Propstat[0].Status)
	}
}

func TestExtractCalendarsFromResponse(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/" xmlns:A="http://apple.com/ns/ical/">
	<D:response>
		<D:href>/123456/calendars/home/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Home</D:displayname>
				<C:calendar-description>Personal calendar</C:calendar-description>
				<A:calendar-color>#FF5733</A:calendar-color>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
				<C:supported-calendar-component-set>
					<C:comp name="VEVENT"/>
					<C:comp name="VTODO"/>
				</C:supported-calendar-component-set>
				<CS:getctag>123456789</CS:getctag>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/123456/calendars/work/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Work</D:displayname>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/123456/calendars/not-a-calendar/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Not a Calendar</D:displayname>
				<D:resourcetype>
					<D:collection/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		t.Fatalf("unexpected error parsing: %v", err)
	}

	calendars := extractCalendarsFromResponse(resp)

	if len(calendars) != 2 {
		t.Errorf("expected 2 calendars, got %d", len(calendars))
	}

	homeCal := calendars[0]
	if homeCal.DisplayName != "Home" {
		t.Errorf("expected display name 'Home', got %s", homeCal.DisplayName)
	}
	if homeCal.Description != "Personal calendar" {
		t.Errorf("expected description 'Personal calendar', got %s", homeCal.Description)
	}
	if homeCal.Color != "#FF5733" {
		t.Errorf("expected color '#FF5733', got %s", homeCal.Color)
	}
	if len(homeCal.SupportedComponents) != 2 {
		t.Errorf("expected 2 supported components, got %d", len(homeCal.SupportedComponents))
	}
	if homeCal.CTag != "123456789" {
		t.Errorf("expected ctag '123456789', got %s", homeCal.CTag)
	}

	workCal := calendars[1]
	if workCal.DisplayName != "Work" {
		t.Errorf("expected display name 'Work', got %s", workCal.DisplayName)
	}
}

func TestExtractCalendarObjectsFromResponse(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
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
DESCRIPTION:Weekly sync
LOCATION:Conference Room A
DTSTART:20250115T140000Z
DTEND:20250115T150000Z
STATUS:CONFIRMED
ORGANIZER:mailto:organizer@example.com
ATTENDEE:mailto:attendee1@example.com
ATTENDEE:mailto:attendee2@example.com
CREATED:20250101T120000Z
LAST-MODIFIED:20250110T090000Z
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/calendars/test/event2.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 404 Not Found</D:status>
			<D:prop/>
		</D:propstat>
	</D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		t.Fatalf("unexpected error parsing: %v", err)
	}

	objects := extractCalendarObjectsFromResponse(resp)

	if len(objects) != 1 {
		t.Errorf("expected 1 calendar object, got %d", len(objects))
	}

	obj := objects[0]
	if obj.Href != "/calendars/test/event1.ics" {
		t.Errorf("expected href '/calendars/test/event1.ics', got %s", obj.Href)
	}
	if obj.ETag != `"abc123"` {
		t.Errorf("expected etag '\"abc123\"', got %s", obj.ETag)
	}
	if obj.UID != "test-uid-123" {
		t.Errorf("expected UID 'test-uid-123', got %s", obj.UID)
	}
	if obj.Summary != "Team Meeting" {
		t.Errorf("expected summary 'Team Meeting', got %s", obj.Summary)
	}
	if obj.Description != "Weekly sync" {
		t.Errorf("expected description 'Weekly sync', got %s", obj.Description)
	}
	if obj.Location != "Conference Room A" {
		t.Errorf("expected location 'Conference Room A', got %s", obj.Location)
	}
	if obj.Status != "CONFIRMED" {
		t.Errorf("expected status 'CONFIRMED', got %s", obj.Status)
	}
	if len(obj.Attendees) != 2 {
		t.Errorf("expected 2 attendees, got %d", len(obj.Attendees))
	}

	if obj.StartTime == nil {
		t.Error("expected start time to be set")
	} else {
		expectedStart := time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC)
		if !obj.StartTime.Equal(expectedStart) {
			t.Errorf("expected start time %v, got %v", expectedStart, obj.StartTime)
		}
	}

	if obj.EndTime == nil {
		t.Error("expected end time to be set")
	}
}

func TestParseStatusCode(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"HTTP/1.1 200 OK", 200},
		{"HTTP/1.1 404 Not Found", 404},
		{"HTTP/1.1 207 Multi-Status", 207},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseStatusCode(tt.input)
			if result != tt.expected {
				t.Errorf("parseStatusCode(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPrincipalFromResponse(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
	<D:response>
		<D:href>/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:current-user-principal>
					<D:href>/123456/principal/</D:href>
				</D:current-user-principal>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		t.Fatalf("unexpected error parsing: %v", err)
	}

	principal := extractPrincipalFromResponse(resp)

	if principal != "/123456/principal/" {
		t.Errorf("expected principal '/123456/principal/', got %s", principal)
	}
}

func TestExtractCalendarHomeSetFromResponse(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/123456/principal/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-home-set>
					<D:href>/123456/calendars/</D:href>
				</C:calendar-home-set>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		t.Fatalf("unexpected error parsing: %v", err)
	}

	homeSet := extractCalendarHomeSetFromResponse(resp)

	if homeSet != "/123456/calendars/" {
		t.Errorf("expected calendar home set '/123456/calendars/', got %s", homeSet)
	}
}
