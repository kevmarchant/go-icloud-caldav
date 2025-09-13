package caldav

import (
	"context"
	"io"
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

func TestParseCalendarMetadata(t *testing.T) {
	responseBody := `<?xml version="1.0" encoding="UTF-8"?>
	<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:a="http://apple.com/ns/ical/">
		<d:response>
			<d:href>/123456789/calendars/home/</d:href>
			<d:propstat>
				<d:status>HTTP/1.1 200 OK</d:status>
				<d:prop>
					<d:displayname>Work Calendar</d:displayname>
					<c:calendar-description>My work calendar</c:calendar-description>
					<a:calendar-color>#FF0000</a:calendar-color>
					<d:resourcetype>
						<d:collection/>
						<c:calendar/>
					</d:resourcetype>
					<c:supported-calendar-component-set>
						<c:comp name="VEVENT"/>
						<c:comp name="VTODO"/>
					</c:supported-calendar-component-set>
					<cs:getctag xmlns:cs="http://calendarserver.org/ns/">12345</cs:getctag>
					<d:getetag>"abcdef"</d:getetag>
					<c:calendar-timezone>America/New_York</c:calendar-timezone>
					<c:max-resource-size>1048576</c:max-resource-size>
					<c:min-date-time>19700101T000000Z</c:min-date-time>
					<c:max-date-time>20381231T235959Z</c:max-date-time>
					<c:max-instances>100</c:max-instances>
					<c:max-attendees-per-instance>25</c:max-attendees-per-instance>
					<d:current-user-privilege-set>
						<d:privilege>
							<d:read/>
						</d:privilege>
						<d:privilege>
							<d:write/>
						</d:privilege>
					</d:current-user-privilege-set>
					<d:supported-report-set>
						<d:supported-report>
							<d:report>
								<c:calendar-query/>
							</d:report>
						</d:supported-report>
						<d:supported-report>
							<d:report>
								<c:calendar-multiget/>
							</d:report>
						</d:supported-report>
					</d:supported-report-set>
					<d:quota-used-bytes>512000</d:quota-used-bytes>
					<d:quota-available-bytes>536576000</d:quota-available-bytes>
				</d:prop>
			</d:propstat>
		</d:response>
	</d:multistatus>`

	resp, err := parseMultiStatusResponse(strings.NewReader(responseBody))
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resp.Responses))
	}

	if len(resp.Responses[0].Propstat) != 1 {
		t.Fatalf("expected 1 propstat, got %d", len(resp.Responses[0].Propstat))
	}

	prop := resp.Responses[0].Propstat[0].Prop

	// Test basic properties
	if prop.DisplayName != "Work Calendar" {
		t.Errorf("expected display name 'Work Calendar', got %s", prop.DisplayName)
	}

	// Test new metadata properties
	if prop.CalendarTimeZone != "America/New_York" {
		t.Errorf("expected calendar timezone 'America/New_York', got %s", prop.CalendarTimeZone)
	}

	if prop.MaxResourceSize != 1048576 {
		t.Errorf("expected max resource size 1048576, got %d", prop.MaxResourceSize)
	}

	if prop.MinDateTime != "19700101T000000Z" {
		t.Errorf("expected min date time '19700101T000000Z', got %s", prop.MinDateTime)
	}

	if prop.MaxDateTime != "20381231T235959Z" {
		t.Errorf("expected max date time '20381231T235959Z', got %s", prop.MaxDateTime)
	}

	if prop.MaxInstances != 100 {
		t.Errorf("expected max instances 100, got %d", prop.MaxInstances)
	}

	if prop.MaxAttendeesPerInstance != 25 {
		t.Errorf("expected max attendees per instance 25, got %d", prop.MaxAttendeesPerInstance)
	}

	if len(prop.CurrentUserPrivilegeSet) != 2 {
		t.Errorf("expected 2 privileges, got %d", len(prop.CurrentUserPrivilegeSet))
	}

	expectedPrivileges := []string{"read", "write"}
	for i, expected := range expectedPrivileges {
		if i >= len(prop.CurrentUserPrivilegeSet) || prop.CurrentUserPrivilegeSet[i] != expected {
			t.Errorf("expected privilege[%d] to be '%s', got '%s'", i, expected, prop.CurrentUserPrivilegeSet[i])
		}
	}

	if len(prop.SupportedReports) != 2 {
		t.Errorf("expected 2 supported reports, got %d", len(prop.SupportedReports))
	}

	expectedReports := []string{"calendar-query", "calendar-multiget"}
	for i, expected := range expectedReports {
		if i >= len(prop.SupportedReports) || prop.SupportedReports[i] != expected {
			t.Errorf("expected supported report[%d] to be '%s', got '%s'", i, expected, prop.SupportedReports[i])
		}
	}

	if prop.QuotaUsedBytes != 512000 {
		t.Errorf("expected quota used bytes 512000, got %d", prop.QuotaUsedBytes)
	}

	if prop.QuotaAvailableBytes != 536576000 {
		t.Errorf("expected quota available bytes 536576000, got %d", prop.QuotaAvailableBytes)
	}
}

func TestParseCalendarMetadataExtract(t *testing.T) {
	responseBody := `<?xml version="1.0" encoding="UTF-8"?>
	<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:a="http://apple.com/ns/ical/">
		<d:response>
			<d:href>/123456789/calendars/work/</d:href>
			<d:propstat>
				<d:status>HTTP/1.1 200 OK</d:status>
				<d:prop>
					<d:displayname>Work Calendar</d:displayname>
					<c:calendar-description>My work calendar</c:calendar-description>
					<a:calendar-color>#FF0000</a:calendar-color>
					<d:resourcetype>
						<d:collection/>
						<c:calendar/>
					</d:resourcetype>
					<c:supported-calendar-component-set>
						<c:comp name="VEVENT"/>
					</c:supported-calendar-component-set>
					<cs:getctag xmlns:cs="http://calendarserver.org/ns/">12345</cs:getctag>
					<d:getetag>"abcdef"</d:getetag>
					<c:calendar-timezone>UTC</c:calendar-timezone>
					<c:max-resource-size>2097152</c:max-resource-size>
					<c:min-date-time>2000-01-01T00:00:00Z</c:min-date-time>
					<c:max-date-time>2030-12-31T23:59:59Z</c:max-date-time>
					<d:quota-used-bytes>1024000</d:quota-used-bytes>
				</d:prop>
			</d:propstat>
		</d:response>
	</d:multistatus>`

	resp, err := parseMultiStatusResponse(strings.NewReader(responseBody))
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	calendars := extractCalendarsFromResponse(resp)
	if len(calendars) != 1 {
		t.Fatalf("expected 1 calendar, got %d", len(calendars))
	}

	cal := calendars[0]

	// Test that metadata is properly extracted to Calendar struct
	if cal.CalendarTimeZone != "UTC" {
		t.Errorf("expected calendar timezone 'UTC', got %s", cal.CalendarTimeZone)
	}

	if cal.MaxResourceSize != 2097152 {
		t.Errorf("expected max resource size 2097152, got %d", cal.MaxResourceSize)
	}

	// Test date parsing
	expectedMinDate := "2000-01-01T00:00:00Z"
	if cal.MinDateTime == nil {
		t.Error("expected min date time to be parsed, got nil")
	} else if cal.MinDateTime.Format(time.RFC3339) != expectedMinDate {
		t.Errorf("expected min date time '%s', got '%s'", expectedMinDate, cal.MinDateTime.Format(time.RFC3339))
	}

	expectedMaxDate := "2030-12-31T23:59:59Z"
	if cal.MaxDateTime == nil {
		t.Error("expected max date time to be parsed, got nil")
	} else if cal.MaxDateTime.Format(time.RFC3339) != expectedMaxDate {
		t.Errorf("expected max date time '%s', got '%s'", expectedMaxDate, cal.MaxDateTime.Format(time.RFC3339))
	}

	if cal.Quota.QuotaUsedBytes != 1024000 {
		t.Errorf("expected quota used bytes 1024000, got %d", cal.Quota.QuotaUsedBytes)
	}
}

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
			input: "DTSTART:not-a-date",
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
