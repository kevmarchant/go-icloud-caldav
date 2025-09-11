package caldav

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func BenchmarkParseMultiStatusResponse(b *testing.B) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Test Calendar</D:displayname>
				<C:calendar-description>Test Description</C:calendar-description>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(xmlData)
		_, err := parseMultiStatusResponse(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMultiStatusResponseLarge(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	for i := 0; i < 100; i++ {
		sb.WriteString(`
	<D:response>
		<D:href>/calendars/test`)
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(`/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Test Calendar `)
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(`</D:displayname>
				<C:calendar-description>Test Description</C:calendar-description>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>`)
	}

	sb.WriteString(`</D:multistatus>`)
	xmlData := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(xmlData)
		_, err := parseMultiStatusResponse(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractCalendarObjects(b *testing.B) {
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
DESCRIPTION:Weekly sync meeting with the entire team
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
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractCalendarObjectsFromResponse(resp)
	}
}

func BenchmarkExtractCalendarObjectsMany(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	for i := 0; i < 100; i++ {
		sb.WriteString(`
	<D:response>
		<D:href>/calendars/test/event`)
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(`.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:getetag>"abc`)
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(`"</D:getetag>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-uid-`)
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(`
SUMMARY:Team Meeting `)
		sb.WriteString(fmt.Sprintf("%d", i))
		sb.WriteString(`
DESCRIPTION:Weekly sync meeting
LOCATION:Conference Room
DTSTART:20250115T140000Z
DTEND:20250115T150000Z
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>`)
	}

	sb.WriteString(`</D:multistatus>`)
	xmlData := sb.String()

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractCalendarObjectsFromResponse(resp)
	}
}

func BenchmarkParseICalTime(b *testing.B) {
	testCases := []string{
		"DTSTART:20250111T123045Z",
		"DTEND:20250111T123045",
		"CREATED:20250111",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = parseICalTime(tc)
		}
	}
}

func BenchmarkBuildPropfindXML(b *testing.B) {
	props := []string{"displayname", "resourcetype", "calendar-home-set"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := buildPropfindXML(props)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildCalendarQueryXML(b *testing.B) {
	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		Filter: Filter{
			Component: "VEVENT",
			TimeRange: &TimeRange{
				Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := buildCalendarQueryXML(query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildCalendarQueryXMLComplex(b *testing.B) {
	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data", "displayname", "resourcetype"},
		Filter: Filter{
			Component: "VEVENT",
			TimeRange: &TimeRange{
				Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			},
			Props: []PropFilter{
				{
					Name: "UID",
					TextMatch: &TextMatch{
						Value: "test-uid",
					},
				},
				{
					Name: "SUMMARY",
					TextMatch: &TextMatch{
						Value:           "meeting",
						Collation:       "i;unicode-casemap",
						NegateCondition: false,
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := buildCalendarQueryXML(query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseStatusCode(b *testing.B) {
	testCases := []string{
		"HTTP/1.1 200 OK",
		"HTTP/1.1 404 Not Found",
		"HTTP/1.1 207 Multi-Status",
		"HTTP/2 201 Created",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = parseStatusCode(tc)
		}
	}
}

func BenchmarkExtractCalendarsFromResponse(b *testing.B) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/home/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Home</D:displayname>
				<C:calendar-description>Personal calendar</C:calendar-description>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>
	<D:response>
		<D:href>/calendars/work/</D:href>
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
</D:multistatus>`

	reader := strings.NewReader(xmlData)
	resp, err := parseMultiStatusResponse(reader)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractCalendarsFromResponse(resp)
	}
}

func BenchmarkMemoryAllocation(b *testing.B) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/event.ics</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-uid
SUMMARY:Test Event
DTSTART:20250115T140000Z
DTEND:20250115T150000Z
END:VEVENT
END:VCALENDAR</C:calendar-data>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(xmlData)
		resp, _ := parseMultiStatusResponse(reader)
		_ = extractCalendarObjectsFromResponse(resp)
	}
}

func BenchmarkConcurrentParsing(b *testing.B) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/calendars/test/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Test Calendar</D:displayname>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			reader := strings.NewReader(xmlData)
			_, err := parseMultiStatusResponse(reader)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
