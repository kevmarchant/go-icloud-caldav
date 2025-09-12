package caldav

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseICalendar_BasicEvent(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
CALSCALE:GREGORIAN
METHOD:PUBLISH
BEGIN:VEVENT
UID:test-uid-123
DTSTAMP:20240101T120000Z
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
DESCRIPTION:This is a test meeting
LOCATION:Conference Room A
STATUS:CONFIRMED
SEQUENCE:0
PRIORITY:5
CLASS:PUBLIC
TRANSP:OPAQUE
CREATED:20240101T090000Z
LAST-MODIFIED:20240101T100000Z
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if parsed == nil {
		t.Fatal("ParseICalendar returned nil")
	}

	if parsed.Version != "2.0" {
		t.Errorf("Version: expected 2.0, got %s", parsed.Version)
	}

	if parsed.ProdID != "-//Test//Test//EN" {
		t.Errorf("ProdID: expected -//Test//Test//EN, got %s", parsed.ProdID)
	}

	if parsed.CalScale != "GREGORIAN" {
		t.Errorf("CalScale: expected GREGORIAN, got %s", parsed.CalScale)
	}

	if parsed.Method != "PUBLISH" {
		t.Errorf("Method: expected PUBLISH, got %s", parsed.Method)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if event.UID != "test-uid-123" {
		t.Errorf("UID: expected test-uid-123, got %s", event.UID)
	}

	if event.Summary != "Test Meeting" {
		t.Errorf("Summary: expected Test Meeting, got %s", event.Summary)
	}

	if event.Description != "This is a test meeting" {
		t.Errorf("Description: expected 'This is a test meeting', got %s", event.Description)
	}

	if event.Location != "Conference Room A" {
		t.Errorf("Location: expected 'Conference Room A', got %s", event.Location)
	}

	if event.Status != "CONFIRMED" {
		t.Errorf("Status: expected CONFIRMED, got %s", event.Status)
	}

	if event.Transparency != "OPAQUE" {
		t.Errorf("Transparency: expected OPAQUE, got %s", event.Transparency)
	}

	if event.Sequence != 0 {
		t.Errorf("Sequence: expected 0, got %d", event.Sequence)
	}

	if event.Priority != 5 {
		t.Errorf("Priority: expected 5, got %d", event.Priority)
	}

	if event.Class != "PUBLIC" {
		t.Errorf("Class: expected PUBLIC, got %s", event.Class)
	}

	expectedStart, _ := time.Parse("20060102T150405Z", "20240115T100000Z")
	if event.DTStart == nil || !event.DTStart.Equal(expectedStart) {
		t.Errorf("DTStart: expected %v, got %v", expectedStart, event.DTStart)
	}

	expectedEnd, _ := time.Parse("20060102T150405Z", "20240115T110000Z")
	if event.DTEnd == nil || !event.DTEnd.Equal(expectedEnd) {
		t.Errorf("DTEnd: expected %v, got %v", expectedEnd, event.DTEnd)
	}
}

func TestParseICalendar_EventWithAttendees(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-uid-456
DTSTAMP:20240101T120000Z
DTSTART:20240115T140000Z
DTEND:20240115T150000Z
SUMMARY:Team Meeting
ORGANIZER;CN=John Doe;EMAIL=john@example.com:mailto:john@example.com
ATTENDEE;CN=Jane Smith;ROLE=REQ-PARTICIPANT;PARTSTAT=ACCEPTED;RSVP=TRUE:mailto:jane@example.com
ATTENDEE;CN=Bob Johnson;ROLE=OPT-PARTICIPANT;PARTSTAT=TENTATIVE;RSVP=FALSE:mailto:bob@example.com
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if event.Organizer.CN != "John Doe" {
		t.Errorf("Organizer CN: expected 'John Doe', got %s", event.Organizer.CN)
	}

	if event.Organizer.Email != "john@example.com" {
		t.Errorf("Organizer Email: expected 'john@example.com', got %s", event.Organizer.Email)
	}

	if len(event.Attendees) != 2 {
		t.Fatalf("Expected 2 attendees, got %d", len(event.Attendees))
	}

	attendee1 := event.Attendees[0]
	if attendee1.CN != "Jane Smith" {
		t.Errorf("Attendee 1 CN: expected 'Jane Smith', got %s", attendee1.CN)
	}
	if attendee1.Email != "jane@example.com" {
		t.Errorf("Attendee 1 Email: expected 'jane@example.com', got %s", attendee1.Email)
	}
	if attendee1.Role != "REQ-PARTICIPANT" {
		t.Errorf("Attendee 1 Role: expected 'REQ-PARTICIPANT', got %s", attendee1.Role)
	}
	if attendee1.PartStat != "ACCEPTED" {
		t.Errorf("Attendee 1 PartStat: expected 'ACCEPTED', got %s", attendee1.PartStat)
	}
	if !attendee1.RSVP {
		t.Error("Attendee 1 RSVP: expected true, got false")
	}

	attendee2 := event.Attendees[1]
	if attendee2.CN != "Bob Johnson" {
		t.Errorf("Attendee 2 CN: expected 'Bob Johnson', got %s", attendee2.CN)
	}
	if attendee2.RSVP {
		t.Error("Attendee 2 RSVP: expected false, got true")
	}
}

func TestParseICalendar_RecurringEvent(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:recurring-event
DTSTART:20240101T090000Z
DTEND:20240101T100000Z
RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=10
SUMMARY:Weekly Standup
CATEGORIES:Meeting,Team
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if event.RecurrenceRule != "FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=10" {
		t.Errorf("RecurrenceRule: expected 'FREQ=WEEKLY;BYDAY=MO,WE,FR;COUNT=10', got %s", event.RecurrenceRule)
	}

	if len(event.Categories) != 2 {
		t.Fatalf("Expected 2 categories, got %d", len(event.Categories))
	}

	expectedCategories := []string{"Meeting", "Team"}
	if !reflect.DeepEqual(event.Categories, expectedCategories) {
		t.Errorf("Categories: expected %v, got %v", expectedCategories, event.Categories)
	}
}

func TestParseICalendar_Todo(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VTODO
UID:todo-001
DTSTAMP:20240101T120000Z
DTSTART:20240115T090000Z
DUE:20240120T170000Z
SUMMARY:Complete project documentation
DESCRIPTION:Write comprehensive documentation for the new feature
STATUS:IN-PROCESS
PERCENT-COMPLETE:75
PRIORITY:1
CATEGORIES:Work,Documentation
CREATED:20240101T090000Z
LAST-MODIFIED:20240110T150000Z
END:VTODO
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Todos) != 1 {
		t.Fatalf("Expected 1 todo, got %d", len(parsed.Todos))
	}

	todo := parsed.Todos[0]

	if todo.UID != "todo-001" {
		t.Errorf("UID: expected 'todo-001', got %s", todo.UID)
	}

	if todo.Summary != "Complete project documentation" {
		t.Errorf("Summary: expected 'Complete project documentation', got %s", todo.Summary)
	}

	if todo.Status != "IN-PROCESS" {
		t.Errorf("Status: expected 'IN-PROCESS', got %s", todo.Status)
	}

	if todo.PercentComplete != 75 {
		t.Errorf("PercentComplete: expected 75, got %d", todo.PercentComplete)
	}

	if todo.Priority != 1 {
		t.Errorf("Priority: expected 1, got %d", todo.Priority)
	}

	expectedDue, _ := time.Parse("20060102T150405Z", "20240120T170000Z")
	if todo.Due == nil || !todo.Due.Equal(expectedDue) {
		t.Errorf("Due: expected %v, got %v", expectedDue, todo.Due)
	}
}

func TestParseICalendar_EventWithAlarm(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:event-with-alarm
DTSTART:20240115T100000Z
SUMMARY:Important Meeting
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:Meeting reminder
END:VALARM
BEGIN:VALARM
ACTION:EMAIL
TRIGGER:-PT1H
SUMMARY:Meeting in 1 hour
DESCRIPTION:Don't forget the meeting
ATTENDEE:mailto:user@example.com
END:VALARM
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if len(event.Alarms) != 2 {
		t.Fatalf("Expected 2 alarms, got %d", len(event.Alarms))
	}

	alarm1 := event.Alarms[0]
	if alarm1.Action != "DISPLAY" {
		t.Errorf("Alarm 1 Action: expected 'DISPLAY', got %s", alarm1.Action)
	}
	if alarm1.Trigger != "-PT15M" {
		t.Errorf("Alarm 1 Trigger: expected '-PT15M', got %s", alarm1.Trigger)
	}
	if alarm1.Description != "Meeting reminder" {
		t.Errorf("Alarm 1 Description: expected 'Meeting reminder', got %s", alarm1.Description)
	}

	alarm2 := event.Alarms[1]
	if alarm2.Action != "EMAIL" {
		t.Errorf("Alarm 2 Action: expected 'EMAIL', got %s", alarm2.Action)
	}
	if len(alarm2.Attendees) != 1 {
		t.Errorf("Alarm 2 Attendees: expected 1, got %d", len(alarm2.Attendees))
	}
}

func TestParseICalendar_MultipleEvents(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event-1
DTSTART:20240101T090000Z
SUMMARY:Event 1
END:VEVENT
BEGIN:VEVENT
UID:event-2
DTSTART:20240102T100000Z
SUMMARY:Event 2
END:VEVENT
BEGIN:VEVENT
UID:event-3
DTSTART:20240103T110000Z
SUMMARY:Event 3
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(parsed.Events))
	}

	expectedUIDs := []string{"event-1", "event-2", "event-3"}
	expectedSummaries := []string{"Event 1", "Event 2", "Event 3"}

	for i, event := range parsed.Events {
		if event.UID != expectedUIDs[i] {
			t.Errorf("Event %d UID: expected %s, got %s", i+1, expectedUIDs[i], event.UID)
		}
		if event.Summary != expectedSummaries[i] {
			t.Errorf("Event %d Summary: expected %s, got %s", i+1, expectedSummaries[i], event.Summary)
		}
	}
}

func TestParseICalendar_LineFolding(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-folding
DTSTART:20240115T100000Z
SUMMARY:This is a very long summary that spans
 multiple lines using the line folding mechanism
 defined in RFC 5545
DESCRIPTION:Another long description
	with tab continuation
	instead of space
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	expectedSummary := "This is a very long summary that spans" +
		"multiple lines using the line folding mechanism" +
		"defined in RFC 5545"
	if event.Summary != expectedSummary {
		t.Errorf("Summary mismatch:\nExpected: %s\nGot: %s", expectedSummary, event.Summary)
	}

	expectedDescription := "Another long description" +
		"with tab continuation" +
		"instead of space"
	if event.Description != expectedDescription {
		t.Errorf("Description mismatch:\nExpected: %s\nGot: %s", expectedDescription, event.Description)
	}
}

func TestParseICalendar_GeoLocation(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:geo-event
DTSTART:20240115T100000Z
SUMMARY:Meeting at specific location
GEO:37.386013;-122.082932
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if event.GeoLocation == nil {
		t.Fatal("GeoLocation is nil")
	}

	expectedLat := 37.386013
	expectedLon := -122.082932

	if event.GeoLocation.Latitude != expectedLat {
		t.Errorf("Latitude: expected %f, got %f", expectedLat, event.GeoLocation.Latitude)
	}

	if event.GeoLocation.Longitude != expectedLon {
		t.Errorf("Longitude: expected %f, got %f", expectedLon, event.GeoLocation.Longitude)
	}
}

func TestParseICalendar_CustomProperties(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-CALNAME:My Calendar
X-WR-TIMEZONE:America/New_York
BEGIN:VEVENT
UID:custom-props
DTSTART:20240115T100000Z
SUMMARY:Event with custom props
X-MICROSOFT-CDO-BUSYSTATUS:BUSY
X-CUSTOM-FIELD:CustomValue
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if parsed.CustomProperties["X-WR-CALNAME"] != "My Calendar" {
		t.Errorf("Calendar custom property X-WR-CALNAME: expected 'My Calendar', got %s",
			parsed.CustomProperties["X-WR-CALNAME"])
	}

	if parsed.CustomProperties["X-WR-TIMEZONE"] != "America/New_York" {
		t.Errorf("Calendar custom property X-WR-TIMEZONE: expected 'America/New_York', got %s",
			parsed.CustomProperties["X-WR-TIMEZONE"])
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if event.CustomProperties["X-MICROSOFT-CDO-BUSYSTATUS"] != "BUSY" {
		t.Errorf("Event custom property X-MICROSOFT-CDO-BUSYSTATUS: expected 'BUSY', got %s",
			event.CustomProperties["X-MICROSOFT-CDO-BUSYSTATUS"])
	}

	if event.CustomProperties["X-CUSTOM-FIELD"] != "CustomValue" {
		t.Errorf("Event custom property X-CUSTOM-FIELD: expected 'CustomValue', got %s",
			event.CustomProperties["X-CUSTOM-FIELD"])
	}
}

func TestParseICalendar_EmptyCalendar(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if parsed == nil {
		t.Fatal("ParseICalendar returned nil")
	}

	if len(parsed.Events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(parsed.Events))
	}

	if len(parsed.Todos) != 0 {
		t.Errorf("Expected 0 todos, got %d", len(parsed.Todos))
	}
}

func TestParseICalendar_MalformedData(t *testing.T) {
	testCases := []struct {
		name        string
		icalData    string
		shouldError bool
	}{
		{
			name:        "Missing BEGIN:VCALENDAR",
			icalData:    "VERSION:2.0\nEND:VCALENDAR",
			shouldError: false, // Parser is lenient
		},
		{
			name:        "Missing END:VCALENDAR",
			icalData:    "BEGIN:VCALENDAR\nVERSION:2.0",
			shouldError: false, // Parser is lenient
		},
		{
			name:        "Empty string",
			icalData:    "",
			shouldError: false, // Parser handles empty input
		},
		{
			name: "Line without colon",
			icalData: `BEGIN:VCALENDAR
VERSION:2.0
INVALID LINE WITHOUT COLON
END:VCALENDAR`,
			shouldError: false, // Parser skips invalid lines
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseICalendar(tc.icalData)
			if tc.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if parsed == nil {
					t.Error("ParseICalendar returned nil without error")
				}
			}
		})
	}
}

func TestExtractCalendarObjectsWithAutoParsing(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/event1.ics</D:href>
    <D:propstat>
      <D:status>HTTP/1.1 200 OK</D:status>
      <D:prop>
        <D:getetag>"12345"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Event
DESCRIPTION:This is a test event
LOCATION:Test Location
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		t.Fatalf("Failed to parse multistatus response: %v", err)
	}

	// Test without auto-parsing
	objectsWithoutParsing := extractCalendarObjectsFromResponseWithOptions(resp, false)
	if len(objectsWithoutParsing) != 1 {
		t.Fatalf("Expected 1 object without parsing, got %d", len(objectsWithoutParsing))
	}

	objWithout := objectsWithoutParsing[0]
	if objWithout.ParsedData != nil {
		t.Error("ParsedData should be nil when auto-parsing is disabled")
	}
	if objWithout.ParseError != nil {
		t.Error("ParseError should be nil when auto-parsing is disabled")
	}

	// Test with auto-parsing
	objectsWithParsing := extractCalendarObjectsFromResponseWithOptions(resp, true)
	if len(objectsWithParsing) != 1 {
		t.Fatalf("Expected 1 object with parsing, got %d", len(objectsWithParsing))
	}

	objWith := objectsWithParsing[0]
	if objWith.ParsedData == nil {
		t.Fatal("ParsedData should not be nil when auto-parsing is enabled")
	}
	if objWith.ParseError != nil {
		t.Errorf("ParseError should be nil for valid data: %v", objWith.ParseError)
	}

	// Verify parsed data
	if len(objWith.ParsedData.Events) != 1 {
		t.Fatalf("Expected 1 parsed event, got %d", len(objWith.ParsedData.Events))
	}

	event := objWith.ParsedData.Events[0]
	if event.UID != "test-event-1" {
		t.Errorf("Parsed event UID: expected 'test-event-1', got %s", event.UID)
	}
	if event.Summary != "Test Event" {
		t.Errorf("Parsed event Summary: expected 'Test Event', got %s", event.Summary)
	}
	if event.Description != "This is a test event" {
		t.Errorf("Parsed event Description: expected 'This is a test event', got %s", event.Description)
	}
	if event.Location != "Test Location" {
		t.Errorf("Parsed event Location: expected 'Test Location', got %s", event.Location)
	}
	if event.Status != "CONFIRMED" {
		t.Errorf("Parsed event Status: expected 'CONFIRMED', got %s", event.Status)
	}
}

func TestExtractCalendarObjectsWithAutoParsing_InvalidData(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/event1.ics</D:href>
    <D:propstat>
      <D:status>HTTP/1.1 200 OK</D:status>
      <D:prop>
        <D:getetag>"12345"</D:getetag>
        <C:calendar-data>This is not valid iCalendar data</C:calendar-data>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		t.Fatalf("Failed to parse multistatus response: %v", err)
	}

	// Test with auto-parsing on invalid data
	objects := extractCalendarObjectsFromResponseWithOptions(resp, true)
	if len(objects) != 1 {
		t.Fatalf("Expected 1 object, got %d", len(objects))
	}

	obj := objects[0]
	// Even with invalid data, we should have a ParsedData object (but it may be empty)
	if obj.ParsedData == nil {
		t.Fatal("ParsedData should not be nil even for invalid data")
	}
	// ParseError should be nil because our parser is lenient
	if obj.ParseError != nil {
		t.Errorf("ParseError should be nil for lenient parser: %v", obj.ParseError)
	}
}

func TestParseICalendar_Journal(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VJOURNAL
UID:journal-001
DTSTAMP:20240101T120000Z
DTSTART:20240115T090000Z
SUMMARY:Daily Journal Entry
DESCRIPTION:Today was a productive day
STATUS:FINAL
CATEGORIES:Work,Personal
CREATED:20240115T080000Z
LAST-MODIFIED:20240115T085000Z
SEQUENCE:1
CLASS:PRIVATE
END:VJOURNAL
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Journals) != 1 {
		t.Fatalf("Expected 1 journal, got %d", len(parsed.Journals))
	}

	journal := parsed.Journals[0]

	if journal.UID != "journal-001" {
		t.Errorf("UID: expected 'journal-001', got %s", journal.UID)
	}

	if journal.Summary != "Daily Journal Entry" {
		t.Errorf("Summary: expected 'Daily Journal Entry', got %s", journal.Summary)
	}

	if journal.Description != "Today was a productive day" {
		t.Errorf("Description: expected 'Today was a productive day', got %s", journal.Description)
	}

	if journal.Status != "FINAL" {
		t.Errorf("Status: expected 'FINAL', got %s", journal.Status)
	}

	if journal.Sequence != 1 {
		t.Errorf("Sequence: expected 1, got %d", journal.Sequence)
	}

	if journal.Class != "PRIVATE" {
		t.Errorf("Class: expected 'PRIVATE', got %s", journal.Class)
	}

	if len(journal.Categories) != 2 {
		t.Fatalf("Expected 2 categories, got %d", len(journal.Categories))
	}

	expectedCategories := []string{"Work", "Personal"}
	if !reflect.DeepEqual(journal.Categories, expectedCategories) {
		t.Errorf("Categories: expected %v, got %v", expectedCategories, journal.Categories)
	}

	expectedDTStart, _ := time.Parse("20060102T150405Z", "20240115T090000Z")
	if journal.DTStart == nil || !journal.DTStart.Equal(expectedDTStart) {
		t.Errorf("DTStart: expected %v, got %v", expectedDTStart, journal.DTStart)
	}

	expectedCreated, _ := time.Parse("20060102T150405Z", "20240115T080000Z")
	if journal.Created == nil || !journal.Created.Equal(expectedCreated) {
		t.Errorf("Created: expected %v, got %v", expectedCreated, journal.Created)
	}

	expectedLastModified, _ := time.Parse("20060102T150405Z", "20240115T085000Z")
	if journal.LastModified == nil || !journal.LastModified.Equal(expectedLastModified) {
		t.Errorf("LastModified: expected %v, got %v", expectedLastModified, journal.LastModified)
	}
}

func TestParseICalendar_FreeBusy(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VFREEBUSY
UID:freebusy-001
DTSTAMP:20240101T120000Z
DTSTART:20240115T000000Z
DTEND:20240116T000000Z
ORGANIZER;CN=John Doe;EMAIL=john@example.com:mailto:john@example.com
ATTENDEE;CN=Jane Smith;PARTSTAT=ACCEPTED:mailto:jane@example.com
FREEBUSY;FBTYPE=BUSY:20240115T090000Z/20240115T100000Z
FREEBUSY;FBTYPE=FREE:20240115T100000Z/20240115T120000Z
FREEBUSY:20240115T140000Z/20240115T150000Z
END:VFREEBUSY
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.FreeBusy) != 1 {
		t.Fatalf("Expected 1 freebusy, got %d", len(parsed.FreeBusy))
	}

	fb := parsed.FreeBusy[0]

	if fb.UID != "freebusy-001" {
		t.Errorf("UID: expected 'freebusy-001', got %s", fb.UID)
	}

	expectedDTStart, _ := time.Parse("20060102T150405Z", "20240115T000000Z")
	if fb.DTStart == nil || !fb.DTStart.Equal(expectedDTStart) {
		t.Errorf("DTStart: expected %v, got %v", expectedDTStart, fb.DTStart)
	}

	expectedDTEnd, _ := time.Parse("20060102T150405Z", "20240116T000000Z")
	if fb.DTEnd == nil || !fb.DTEnd.Equal(expectedDTEnd) {
		t.Errorf("DTEnd: expected %v, got %v", expectedDTEnd, fb.DTEnd)
	}

	if fb.Organizer.CN != "John Doe" {
		t.Errorf("Organizer CN: expected 'John Doe', got %s", fb.Organizer.CN)
	}

	if fb.Organizer.Email != "john@example.com" {
		t.Errorf("Organizer Email: expected 'john@example.com', got %s", fb.Organizer.Email)
	}

	if len(fb.Attendees) != 1 {
		t.Fatalf("Expected 1 attendee, got %d", len(fb.Attendees))
	}

	attendee := fb.Attendees[0]
	if attendee.CN != "Jane Smith" {
		t.Errorf("Attendee CN: expected 'Jane Smith', got %s", attendee.CN)
	}

	if attendee.PartStat != "ACCEPTED" {
		t.Errorf("Attendee PartStat: expected 'ACCEPTED', got %s", attendee.PartStat)
	}

	if len(fb.FreeBusy) != 3 {
		t.Fatalf("Expected 3 free/busy periods, got %d", len(fb.FreeBusy))
	}

	// Test first period (BUSY)
	period1 := fb.FreeBusy[0]
	if period1.FBType != "BUSY" {
		t.Errorf("Period 1 FBType: expected 'BUSY', got %s", period1.FBType)
	}

	expectedStart1, _ := time.Parse("20060102T150405Z", "20240115T090000Z")
	if !period1.Start.Equal(expectedStart1) {
		t.Errorf("Period 1 Start: expected %v, got %v", expectedStart1, period1.Start)
	}

	expectedEnd1, _ := time.Parse("20060102T150405Z", "20240115T100000Z")
	if !period1.End.Equal(expectedEnd1) {
		t.Errorf("Period 1 End: expected %v, got %v", expectedEnd1, period1.End)
	}

	// Test second period (FREE)
	period2 := fb.FreeBusy[1]
	if period2.FBType != "FREE" {
		t.Errorf("Period 2 FBType: expected 'FREE', got %s", period2.FBType)
	}

	// Test third period (default BUSY)
	period3 := fb.FreeBusy[2]
	if period3.FBType != "BUSY" {
		t.Errorf("Period 3 FBType: expected 'BUSY' (default), got %s", period3.FBType)
	}
}

func TestParseICalendar_TimeZone(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VTIMEZONE
TZID:America/New_York
X-LIC-LOCATION:America/New_York
X-CUSTOM-TZ:CustomValue
END:VTIMEZONE
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.TimeZones) != 1 {
		t.Fatalf("Expected 1 timezone, got %d", len(parsed.TimeZones))
	}

	tz := parsed.TimeZones[0]

	if tz.TZID != "America/New_York" {
		t.Errorf("TZID: expected 'America/New_York', got %s", tz.TZID)
	}

	if tz.CustomProperties["X-LIC-LOCATION"] != "America/New_York" {
		t.Errorf("Custom property X-LIC-LOCATION: expected 'America/New_York', got %s",
			tz.CustomProperties["X-LIC-LOCATION"])
	}

	if tz.CustomProperties["X-CUSTOM-TZ"] != "CustomValue" {
		t.Errorf("Custom property X-CUSTOM-TZ: expected 'CustomValue', got %s",
			tz.CustomProperties["X-CUSTOM-TZ"])
	}
}

func TestParseFreeBusyPeriod(t *testing.T) {
	testCases := []struct {
		name       string
		value      string
		params     map[string]string
		expectNil  bool
		expectedFB FreeBusyPeriod
	}{
		{
			name:   "Valid period with FBTYPE",
			value:  "20240115T090000Z/20240115T100000Z",
			params: map[string]string{"FBTYPE": "BUSY"},
			expectedFB: FreeBusyPeriod{
				Start:  mustParseTime("20240115T090000Z"),
				End:    mustParseTime("20240115T100000Z"),
				FBType: "BUSY",
			},
		},
		{
			name:  "Valid period without FBTYPE (default BUSY)",
			value: "20240115T140000Z/20240115T150000Z",
			expectedFB: FreeBusyPeriod{
				Start:  mustParseTime("20240115T140000Z"),
				End:    mustParseTime("20240115T150000Z"),
				FBType: "BUSY",
			},
		},
		{
			name:      "Invalid format - no slash",
			value:     "20240115T090000Z",
			expectNil: true,
		},
		{
			name:      "Invalid format - too many parts",
			value:     "20240115T090000Z/20240115T100000Z/extra",
			expectNil: true,
		},
		{
			name:      "Invalid start time",
			value:     "invalid-start/20240115T100000Z",
			expectNil: true,
		},
		{
			name:      "Invalid end time",
			value:     "20240115T090000Z/invalid-end",
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &icalParser{}
			result := parser.parseFreeBusyPeriod(tc.value, tc.params)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if !result.Start.Equal(tc.expectedFB.Start) {
				t.Errorf("Start time: expected %v, got %v", tc.expectedFB.Start, result.Start)
			}

			if !result.End.Equal(tc.expectedFB.End) {
				t.Errorf("End time: expected %v, got %v", tc.expectedFB.End, result.End)
			}

			if result.FBType != tc.expectedFB.FBType {
				t.Errorf("FBType: expected %s, got %s", tc.expectedFB.FBType, result.FBType)
			}
		})
	}
}

func TestParseAttendee_EdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		value            string
		params           map[string]string
		expectedAttendee ParsedAttendee
	}{
		{
			name:  "All parameters",
			value: "mailto:attendee@example.com",
			params: map[string]string{
				"CN":             "Full Name",
				"EMAIL":          "email@example.com",
				"ROLE":           "REQ-PARTICIPANT",
				"PARTSTAT":       "ACCEPTED",
				"RSVP":           "TRUE",
				"CUTYPE":         "INDIVIDUAL",
				"MEMBER":         "group@example.com",
				"DELEGATED-TO":   "delegate-to@example.com",
				"DELEGATED-FROM": "delegate-from@example.com",
				"DIR":            "ldap://example.com",
				"SENT-BY":        "sent-by@example.com",
				"X-CUSTOM":       "custom-value",
			},
			expectedAttendee: ParsedAttendee{
				Value:         "mailto:attendee@example.com",
				Email:         "attendee@example.com", // Extracted from mailto:
				CN:            "Full Name",
				Role:          "REQ-PARTICIPANT",
				PartStat:      "ACCEPTED",
				RSVP:          true,
				CUType:        "INDIVIDUAL",
				Member:        "group@example.com",
				DelegatedTo:   "delegate-to@example.com",
				DelegatedFrom: "delegate-from@example.com",
				Dir:           "ldap://example.com",
				SentBy:        "sent-by@example.com",
				CustomParams: map[string]string{
					"X-CUSTOM": "custom-value",
				},
			},
		},
		{
			name:  "RSVP false case",
			value: "user@example.com",
			params: map[string]string{
				"RSVP": "FALSE",
			},
			expectedAttendee: ParsedAttendee{
				Value:        "user@example.com",
				RSVP:         false,
				CustomParams: map[string]string{},
			},
		},
		{
			name:  "No mailto prefix",
			value: "plain-email@example.com",
			params: map[string]string{
				"CN": "Plain User",
			},
			expectedAttendee: ParsedAttendee{
				Value:        "plain-email@example.com",
				CN:           "Plain User",
				CustomParams: map[string]string{},
			},
		},
		{
			name:   "Mixed case mailto",
			value:  "MAILTO:UPPER@EXAMPLE.COM",
			params: map[string]string{},
			expectedAttendee: ParsedAttendee{
				Value:        "MAILTO:UPPER@EXAMPLE.COM",
				Email:        "UPPER@EXAMPLE.COM",
				CustomParams: map[string]string{},
			},
		},
		{
			name:  "EMAIL param with non-mailto value",
			value: "user@example.com",
			params: map[string]string{
				"EMAIL": "different@example.com",
				"CN":    "User Name",
			},
			expectedAttendee: ParsedAttendee{
				Value:        "user@example.com",
				CN:           "User Name",
				Email:        "different@example.com",
				CustomParams: map[string]string{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &icalParser{}
			result := parser.parseAttendee(tc.value, tc.params)

			if result.Value != tc.expectedAttendee.Value {
				t.Errorf("Value: expected %s, got %s", tc.expectedAttendee.Value, result.Value)
			}
			if result.CN != tc.expectedAttendee.CN {
				t.Errorf("CN: expected %s, got %s", tc.expectedAttendee.CN, result.CN)
			}
			if result.Email != tc.expectedAttendee.Email {
				t.Errorf("Email: expected %s, got %s", tc.expectedAttendee.Email, result.Email)
			}
			if result.Role != tc.expectedAttendee.Role {
				t.Errorf("Role: expected %s, got %s", tc.expectedAttendee.Role, result.Role)
			}
			if result.PartStat != tc.expectedAttendee.PartStat {
				t.Errorf("PartStat: expected %s, got %s", tc.expectedAttendee.PartStat, result.PartStat)
			}
			if result.RSVP != tc.expectedAttendee.RSVP {
				t.Errorf("RSVP: expected %t, got %t", tc.expectedAttendee.RSVP, result.RSVP)
			}
			if result.CUType != tc.expectedAttendee.CUType {
				t.Errorf("CUType: expected %s, got %s", tc.expectedAttendee.CUType, result.CUType)
			}
			if result.Member != tc.expectedAttendee.Member {
				t.Errorf("Member: expected %s, got %s", tc.expectedAttendee.Member, result.Member)
			}
			if result.DelegatedTo != tc.expectedAttendee.DelegatedTo {
				t.Errorf("DelegatedTo: expected %s, got %s", tc.expectedAttendee.DelegatedTo, result.DelegatedTo)
			}
			if result.DelegatedFrom != tc.expectedAttendee.DelegatedFrom {
				t.Errorf("DelegatedFrom: expected %s, got %s", tc.expectedAttendee.DelegatedFrom, result.DelegatedFrom)
			}
			if result.Dir != tc.expectedAttendee.Dir {
				t.Errorf("Dir: expected %s, got %s", tc.expectedAttendee.Dir, result.Dir)
			}
			if result.SentBy != tc.expectedAttendee.SentBy {
				t.Errorf("SentBy: expected %s, got %s", tc.expectedAttendee.SentBy, result.SentBy)
			}
			if !reflect.DeepEqual(result.CustomParams, tc.expectedAttendee.CustomParams) {
				t.Errorf("CustomParams: expected %v, got %v", tc.expectedAttendee.CustomParams, result.CustomParams)
			}
		})
	}
}

func TestAppendEventDescription(t *testing.T) {
	testCases := []struct {
		name                string
		initialDescription  string
		appendValue         string
		expectedDescription string
	}{
		{
			name:                "Append to empty description",
			initialDescription:  "",
			appendValue:         "First line",
			expectedDescription: "First line",
		},
		{
			name:                "Append to existing description",
			initialDescription:  "Initial content",
			appendValue:         " - additional content",
			expectedDescription: "Initial content - additional content",
		},
		{
			name:                "Append empty string",
			initialDescription:  "Initial content",
			appendValue:         "",
			expectedDescription: "Initial content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &icalParser{
				currentEvent: &ParsedEvent{
					Description: tc.initialDescription,
				},
				inEvent: true,
			}

			parser.appendEventDescription(tc.appendValue)

			if parser.currentEvent.Description != tc.expectedDescription {
				t.Errorf("Description: expected %s, got %s", tc.expectedDescription, parser.currentEvent.Description)
			}
		})
	}
}

func TestParseGeo_EdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		value       string
		expectNil   bool
		expectedGeo *GeoLocation
	}{
		{
			name:  "Valid coordinates",
			value: "40.7128;-74.0060",
			expectedGeo: &GeoLocation{
				Latitude:  40.7128,
				Longitude: -74.0060,
			},
		},
		{
			name:  "Zero coordinates",
			value: "0;0",
			expectedGeo: &GeoLocation{
				Latitude:  0,
				Longitude: 0,
			},
		},
		{
			name:      "Only one coordinate",
			value:     "40.7128",
			expectNil: true,
		},
		{
			name:      "Too many coordinates",
			value:     "40.7128;-74.0060;100",
			expectNil: true,
		},
		{
			name:      "Invalid latitude",
			value:     "invalid;-74.0060",
			expectNil: true,
		},
		{
			name:      "Invalid longitude",
			value:     "40.7128;invalid",
			expectNil: true,
		},
		{
			name:      "Empty string",
			value:     "",
			expectNil: true,
		},
		{
			name:      "No semicolon separator",
			value:     "40.7128,-74.0060",
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &icalParser{}
			result := parser.parseGeo(tc.value)

			if tc.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if result.Latitude != tc.expectedGeo.Latitude {
				t.Errorf("Latitude: expected %f, got %f", tc.expectedGeo.Latitude, result.Latitude)
			}

			if result.Longitude != tc.expectedGeo.Longitude {
				t.Errorf("Longitude: expected %f, got %f", tc.expectedGeo.Longitude, result.Longitude)
			}
		})
	}
}

func TestHandleAlarmProperty_EdgeCases(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-alarm-edge-cases
DTSTART:20240115T100000Z
SUMMARY:Event with complex alarm
BEGIN:VALARM
ACTION:AUDIO
TRIGGER:-PT30M
DURATION:PT10M
REPEAT:3
DESCRIPTION:Audio alarm description
SUMMARY:Audio alarm summary
ATTENDEE:mailto:alarm-attendee@example.com
X-CUSTOM-ALARM:custom-alarm-value
END:VALARM
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]
	if len(event.Alarms) != 1 {
		t.Fatalf("Expected 1 alarm, got %d", len(event.Alarms))
	}

	alarm := event.Alarms[0]

	if alarm.Action != "AUDIO" {
		t.Errorf("Action: expected 'AUDIO', got %s", alarm.Action)
	}

	if alarm.Trigger != "-PT30M" {
		t.Errorf("Trigger: expected '-PT30M', got %s", alarm.Trigger)
	}

	if alarm.Duration != "PT10M" {
		t.Errorf("Duration: expected 'PT10M', got %s", alarm.Duration)
	}

	if alarm.Repeat != 3 {
		t.Errorf("Repeat: expected 3, got %d", alarm.Repeat)
	}

	if alarm.Description != "Audio alarm description" {
		t.Errorf("Description: expected 'Audio alarm description', got %s", alarm.Description)
	}

	if alarm.Summary != "Audio alarm summary" {
		t.Errorf("Summary: expected 'Audio alarm summary', got %s", alarm.Summary)
	}

	if len(alarm.Attendees) != 1 {
		t.Fatalf("Expected 1 alarm attendee, got %d", len(alarm.Attendees))
	}

	attendee := alarm.Attendees[0]
	if attendee.Email != "alarm-attendee@example.com" {
		t.Errorf("Attendee Email: expected 'alarm-attendee@example.com', got %s", attendee.Email)
	}

	if alarm.CustomProperties["X-CUSTOM-ALARM"] != "custom-alarm-value" {
		t.Errorf("Custom property: expected 'custom-alarm-value', got %s",
			alarm.CustomProperties["X-CUSTOM-ALARM"])
	}
}

func TestParseOrganizer_EdgeCases(t *testing.T) {
	testCases := []struct {
		name              string
		value             string
		params            map[string]string
		expectedOrganizer ParsedOrganizer
	}{
		{
			name:  "All parameters with mailto",
			value: "mailto:organizer@example.com",
			params: map[string]string{
				"CN":       "Organizer Name",
				"EMAIL":    "email-param@example.com",
				"DIR":      "ldap://directory.example.com",
				"SENT-BY":  "assistant@example.com",
				"X-CUSTOM": "custom-organizer-value",
			},
			expectedOrganizer: ParsedOrganizer{
				Value:  "mailto:organizer@example.com",
				Email:  "organizer@example.com", // Extracted from mailto:
				CN:     "Organizer Name",
				Dir:    "ldap://directory.example.com",
				SentBy: "assistant@example.com",
				CustomParams: map[string]string{
					"X-CUSTOM": "custom-organizer-value",
				},
			},
		},
		{
			name:  "Plain email without mailto",
			value: "plain@example.com",
			params: map[string]string{
				"CN": "Plain Organizer",
			},
			expectedOrganizer: ParsedOrganizer{
				Value:        "plain@example.com",
				CN:           "Plain Organizer",
				CustomParams: map[string]string{},
			},
		},
		{
			name:   "Mixed case MAILTO",
			value:  "MAILTO:UPPER@EXAMPLE.COM",
			params: map[string]string{},
			expectedOrganizer: ParsedOrganizer{
				Value:        "MAILTO:UPPER@EXAMPLE.COM",
				Email:        "UPPER@EXAMPLE.COM",
				CustomParams: map[string]string{},
			},
		},
		{
			name:  "EMAIL param with non-mailto value",
			value: "organizer-id-123",
			params: map[string]string{
				"EMAIL": "param-email@example.com",
				"CN":    "ID-based Organizer",
			},
			expectedOrganizer: ParsedOrganizer{
				Value:        "organizer-id-123",
				Email:        "param-email@example.com",
				CN:           "ID-based Organizer",
				CustomParams: map[string]string{},
			},
		},
		{
			name:   "No params",
			value:  "basic-organizer",
			params: map[string]string{},
			expectedOrganizer: ParsedOrganizer{
				Value:        "basic-organizer",
				CustomParams: map[string]string{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &icalParser{}
			result := parser.parseOrganizer(tc.value, tc.params)

			if result.Value != tc.expectedOrganizer.Value {
				t.Errorf("Value: expected %s, got %s", tc.expectedOrganizer.Value, result.Value)
			}
			if result.CN != tc.expectedOrganizer.CN {
				t.Errorf("CN: expected %s, got %s", tc.expectedOrganizer.CN, result.CN)
			}
			if result.Email != tc.expectedOrganizer.Email {
				t.Errorf("Email: expected %s, got %s", tc.expectedOrganizer.Email, result.Email)
			}
			if result.Dir != tc.expectedOrganizer.Dir {
				t.Errorf("Dir: expected %s, got %s", tc.expectedOrganizer.Dir, result.Dir)
			}
			if result.SentBy != tc.expectedOrganizer.SentBy {
				t.Errorf("SentBy: expected %s, got %s", tc.expectedOrganizer.SentBy, result.SentBy)
			}
			if !reflect.DeepEqual(result.CustomParams, tc.expectedOrganizer.CustomParams) {
				t.Errorf("CustomParams: expected %v, got %v", tc.expectedOrganizer.CustomParams, result.CustomParams)
			}
		})
	}
}

func TestHandleTodoProperty_EdgeCases(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VTODO
UID:comprehensive-todo
DTSTAMP:20240101T120000Z
DTSTART:20240115T090000Z
DUE:20240120T170000Z
COMPLETED:20240118T160000Z
SUMMARY:Comprehensive Todo Task
DESCRIPTION:This is a detailed todo description
STATUS:COMPLETED
PERCENT-COMPLETE:100
PRIORITY:1
CATEGORIES:Work,Project,High-Priority
CREATED:20240101T080000Z
LAST-MODIFIED:20240118T160000Z
SEQUENCE:3
CLASS:CONFIDENTIAL
URL:https://example.com/todo/123
X-CUSTOM-TODO:custom-todo-value
X-ANOTHER-PROP:another-value
END:VTODO
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Todos) != 1 {
		t.Fatalf("Expected 1 todo, got %d", len(parsed.Todos))
	}

	todo := parsed.Todos[0]

	// Test all basic string properties
	if todo.UID != "comprehensive-todo" {
		t.Errorf("UID: expected 'comprehensive-todo', got %s", todo.UID)
	}
	if todo.Summary != "Comprehensive Todo Task" {
		t.Errorf("Summary: expected 'Comprehensive Todo Task', got %s", todo.Summary)
	}
	if todo.Description != "This is a detailed todo description" {
		t.Errorf("Description: expected 'This is a detailed todo description', got %s", todo.Description)
	}
	if todo.Status != "COMPLETED" {
		t.Errorf("Status: expected 'COMPLETED', got %s", todo.Status)
	}
	if todo.Class != "CONFIDENTIAL" {
		t.Errorf("Class: expected 'CONFIDENTIAL', got %s", todo.Class)
	}
	if todo.URL != "https://example.com/todo/123" {
		t.Errorf("URL: expected 'https://example.com/todo/123', got %s", todo.URL)
	}

	// Test integer properties
	if todo.PercentComplete != 100 {
		t.Errorf("PercentComplete: expected 100, got %d", todo.PercentComplete)
	}
	if todo.Priority != 1 {
		t.Errorf("Priority: expected 1, got %d", todo.Priority)
	}
	if todo.Sequence != 3 {
		t.Errorf("Sequence: expected 3, got %d", todo.Sequence)
	}

	// Test time properties
	expectedDTStamp, _ := time.Parse("20060102T150405Z", "20240101T120000Z")
	if todo.DTStamp == nil || !todo.DTStamp.Equal(expectedDTStamp) {
		t.Errorf("DTStamp: expected %v, got %v", expectedDTStamp, todo.DTStamp)
	}

	expectedDTStart, _ := time.Parse("20060102T150405Z", "20240115T090000Z")
	if todo.DTStart == nil || !todo.DTStart.Equal(expectedDTStart) {
		t.Errorf("DTStart: expected %v, got %v", expectedDTStart, todo.DTStart)
	}

	expectedDue, _ := time.Parse("20060102T150405Z", "20240120T170000Z")
	if todo.Due == nil || !todo.Due.Equal(expectedDue) {
		t.Errorf("Due: expected %v, got %v", expectedDue, todo.Due)
	}

	expectedCompleted, _ := time.Parse("20060102T150405Z", "20240118T160000Z")
	if todo.Completed == nil || !todo.Completed.Equal(expectedCompleted) {
		t.Errorf("Completed: expected %v, got %v", expectedCompleted, todo.Completed)
	}

	expectedCreated, _ := time.Parse("20060102T150405Z", "20240101T080000Z")
	if todo.Created == nil || !todo.Created.Equal(expectedCreated) {
		t.Errorf("Created: expected %v, got %v", expectedCreated, todo.Created)
	}

	expectedLastModified, _ := time.Parse("20060102T150405Z", "20240118T160000Z")
	if todo.LastModified == nil || !todo.LastModified.Equal(expectedLastModified) {
		t.Errorf("LastModified: expected %v, got %v", expectedLastModified, todo.LastModified)
	}

	// Test categories
	expectedCategories := []string{"Work", "Project", "High-Priority"}
	if !reflect.DeepEqual(todo.Categories, expectedCategories) {
		t.Errorf("Categories: expected %v, got %v", expectedCategories, todo.Categories)
	}

	// Test custom properties
	if todo.CustomProperties["X-CUSTOM-TODO"] != "custom-todo-value" {
		t.Errorf("Custom property X-CUSTOM-TODO: expected 'custom-todo-value', got %s",
			todo.CustomProperties["X-CUSTOM-TODO"])
	}
	if todo.CustomProperties["X-ANOTHER-PROP"] != "another-value" {
		t.Errorf("Custom property X-ANOTHER-PROP: expected 'another-value', got %s",
			todo.CustomProperties["X-ANOTHER-PROP"])
	}
}

func TestHandleEventProperty_EdgeCases(t *testing.T) {
	// Test URL property and other less common event properties
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:event-with-url
DTSTART:20240115T100000Z
SUMMARY:Event with URL and other properties
URL:https://example.com/event/123
DURATION:PT1H30M
RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=5
RECURRENCE-ID:20240122T100000Z
EXDATE:20240129T100000Z
EXDATE:20240205T100000Z
X-CUSTOM-EVENT:custom-event-value
END:VEVENT
END:VCALENDAR`

	parsed, err := ParseICalendar(icalData)
	if err != nil {
		t.Fatalf("ParseICalendar failed: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(parsed.Events))
	}

	event := parsed.Events[0]

	if event.URL != "https://example.com/event/123" {
		t.Errorf("URL: expected 'https://example.com/event/123', got %s", event.URL)
	}

	if event.Duration != "PT1H30M" {
		t.Errorf("Duration: expected 'PT1H30M', got %s", event.Duration)
	}

	if event.RecurrenceRule != "FREQ=WEEKLY;BYDAY=MO;COUNT=5" {
		t.Errorf("RecurrenceRule: expected 'FREQ=WEEKLY;BYDAY=MO;COUNT=5', got %s", event.RecurrenceRule)
	}

	expectedRecurrenceID, _ := time.Parse("20060102T150405Z", "20240122T100000Z")
	if event.RecurrenceID == nil || !event.RecurrenceID.Equal(expectedRecurrenceID) {
		t.Errorf("RecurrenceID: expected %v, got %v", expectedRecurrenceID, event.RecurrenceID)
	}

	// Test exception dates
	if len(event.ExceptionDates) != 2 {
		t.Fatalf("Expected 2 exception dates, got %d", len(event.ExceptionDates))
	}

	expectedExDate1, _ := time.Parse("20060102T150405Z", "20240129T100000Z")
	if !event.ExceptionDates[0].Equal(expectedExDate1) {
		t.Errorf("ExceptionDates[0]: expected %v, got %v", expectedExDate1, event.ExceptionDates[0])
	}

	expectedExDate2, _ := time.Parse("20060102T150405Z", "20240205T100000Z")
	if !event.ExceptionDates[1].Equal(expectedExDate2) {
		t.Errorf("ExceptionDates[1]: expected %v, got %v", expectedExDate2, event.ExceptionDates[1])
	}

	if event.CustomProperties["X-CUSTOM-EVENT"] != "custom-event-value" {
		t.Errorf("Custom property: expected 'custom-event-value', got %s",
			event.CustomProperties["X-CUSTOM-EVENT"])
	}
}

// Helper function for tests
func mustParseTime(value string) time.Time {
	t, err := time.Parse("20060102T150405Z", value)
	if err != nil {
		panic(err)
	}
	return t
}
