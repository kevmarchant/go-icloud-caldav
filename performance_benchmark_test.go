package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func generateLargeMultiStatusResponse(numResponses int) string {
	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	for i := 0; i < numResponses; i++ {
		buf.WriteString(`
  <D:response>
    <D:href>/calendars/test/event`)
		buf.WriteString(intToString(i))
		buf.WriteString(`.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag-`)
		buf.WriteString(intToString(i))
		buf.WriteString(`"</D:getetag>
        <D:displayname>Event `)
		buf.WriteString(intToString(i))
		buf.WriteString(`</D:displayname>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event-`)
		buf.WriteString(intToString(i))
		buf.WriteString(`
DTSTART:20240101T100000Z
DTEND:20240101T110000Z
SUMMARY:Test Event `)
		buf.WriteString(intToString(i))
		buf.WriteString(`
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`)
	}

	buf.WriteString(`
</D:multistatus>`)
	return buf.String()
}

func BenchmarkParseMultiStatusResponsePerformance(b *testing.B) {
	sizes := []int{10, 100, 500, 1000}

	for _, size := range sizes {
		xmlData := generateLargeMultiStatusResponse(size)

		b.Run("XMLParser-"+intToString(size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				reader := strings.NewReader(xmlData)
				_, err := parseMultiStatusResponse(reader)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSyncAllCalendars(b *testing.B) {
	numCalendars := []int{3, 10, 20}

	for _, numCals := range numCalendars {
		server := createMockSyncServer(numCals)
		defer server.Close()

		client := NewClient("test", "test")
		client.baseURL = server.URL

		b.Run("Sequential-"+intToString(numCals), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := client.SyncAllCalendars(context.Background(), nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("Parallel-"+intToString(numCals), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := client.SyncAllCalendarsWithWorkers(context.Background(), nil, 5)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func createMockSyncServer(numCalendars int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == "PROPFIND" {
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/</D:href>
    <D:propstat>
      <D:prop>
        <D:current-user-principal>
          <D:href>/principals/test/</D:href>
        </D:current-user-principal>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			return
		}

		if r.Method == "PROPFIND" && r.URL.Path == "/principals/test/" {
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/principals/test/</D:href>
    <D:propstat>
      <D:prop>
        <D:calendar-home-set>
          <D:href>/calendars/test/</D:href>
        </D:calendar-home-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			return
		}

		if r.Method == "PROPFIND" && strings.Contains(r.URL.Path, "/calendars/") {
			var calendarsXML strings.Builder
			calendarsXML.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

			for i := 0; i < numCalendars; i++ {
				calendarsXML.WriteString(`
  <D:response>
    <D:href>/calendars/test/calendar`)
				calendarsXML.WriteString(intToString(i))
				calendarsXML.WriteString(`/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <D:displayname>Calendar `)
				calendarsXML.WriteString(intToString(i))
				calendarsXML.WriteString(`</D:displayname>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>`)
			}

			calendarsXML.WriteString(`
</D:multistatus>`)

			w.WriteHeader(207)
			_, _ = w.Write([]byte(calendarsXML.String()))
			return
		}

		if r.Method == "REPORT" {
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>sync-token-123</D:sync-token>
  <D:response>
    <D:href>/calendars/test/event.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag-123"</D:getetag>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			return
		}

		w.WriteHeader(404)
	}))
}

func BenchmarkXMLParsing(b *testing.B) {
	xmlData := generateLargeMultiStatusResponse(100)
	reader := bytes.NewReader([]byte(xmlData))

	b.Run("xml.Decoder", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader.Reset([]byte(xmlData))
			_, err := parseMultiStatusResponse(reader)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("xml.Unmarshal", func(b *testing.B) {
		xmlBytes := []byte(xmlData)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var ms xmlMultiStatus
			err := xml.Unmarshal(xmlBytes, &ms)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
