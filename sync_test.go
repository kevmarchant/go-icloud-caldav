package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncCalendar(t *testing.T) {
	tests := []struct {
		name          string
		request       *SyncRequest
		serverStatus  int
		serverResp    string
		expectedError bool
		errorType     ErrorType
		expectedToken string
		expectedCount int
	}{
		{
			name: "initial sync success",
			request: &SyncRequest{
				CalendarURL: "/calendars/user/default/",
				SyncToken:   "",
				SyncLevel:   1,
				Properties:  []string{"getetag", "calendar-data"},
			},
			serverStatus: 207,
			serverResp: `<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>http://example.com/ns/sync/1234</D:sync-token>
  <D:response>
    <D:href>/calendars/user/default/event1.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag1"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:event1
SUMMARY:Test Event 1
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`,
			expectedError: false,
			expectedToken: "http://example.com/ns/sync/1234",
			expectedCount: 1,
		},
		{
			name: "incremental sync with changes",
			request: &SyncRequest{
				CalendarURL: "/calendars/user/default/",
				SyncToken:   "http://example.com/ns/sync/1234",
				SyncLevel:   1,
				Properties:  []string{"getetag", "calendar-data"},
			},
			serverStatus: 207,
			serverResp: `<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>http://example.com/ns/sync/1235</D:sync-token>
  <D:response>
    <D:href>/calendars/user/default/event2.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag2"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:event2
SUMMARY:New Event
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/calendars/user/default/event3.ics</D:href>
    <D:propstat>
      <D:prop/>
      <D:status>HTTP/1.1 404 Not Found</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`,
			expectedError: false,
			expectedToken: "http://example.com/ns/sync/1235",
			expectedCount: 2,
		},
		{
			name: "missing calendar URL",
			request: &SyncRequest{
				CalendarURL: "",
				SyncToken:   "",
				SyncLevel:   1,
			},
			expectedError: true,
			errorType:     ErrorTypeValidation,
		},
		{
			name: "server error",
			request: &SyncRequest{
				CalendarURL: "/calendars/user/default/",
				SyncToken:   "",
				SyncLevel:   1,
			},
			serverStatus:  500,
			expectedError: true,
			errorType:     ErrorTypeServer,
		},
		{
			name: "invalid XML response",
			request: &SyncRequest{
				CalendarURL: "/calendars/user/default/",
				SyncToken:   "",
				SyncLevel:   1,
			},
			serverStatus:  207,
			serverResp:    `invalid xml`,
			expectedError: true,
			errorType:     ErrorTypeInvalidXML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "REPORT" {
					t.Errorf("Expected REPORT method, got %s", r.Method)
				}

				w.WriteHeader(tt.serverStatus)
				if tt.serverResp != "" {
					_, _ = w.Write([]byte(tt.serverResp))
				}
			}))
			defer server.Close()

			client := NewClient("test", "test")
			client.baseURL = server.URL

			resp, err := client.SyncCalendar(context.Background(), tt.request)

			if tt.expectedError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorType != 0 {
					if calErr, ok := err.(*CalDAVError); ok {
						if calErr.Type != tt.errorType {
							t.Errorf("Expected error type %v, got %v", tt.errorType, calErr.Type)
						}
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp.SyncToken != tt.expectedToken {
				t.Errorf("Expected sync token %s, got %s", tt.expectedToken, resp.SyncToken)
			}

			if len(resp.Changes) != tt.expectedCount {
				t.Errorf("Expected %d changes, got %d", tt.expectedCount, len(resp.Changes))
			}
		})
	}
}

func TestInitialSync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>initial-token</D:sync-token>
  <D:response>
    <D:href>/event.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag"</D:getetag>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	resp, err := client.InitialSync(context.Background(), "/calendars/user/")
	if err != nil {
		t.Fatalf("InitialSync failed: %v", err)
	}

	if resp.SyncToken != "initial-token" {
		t.Errorf("Expected sync token 'initial-token', got %s", resp.SyncToken)
	}

	if len(resp.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(resp.Changes))
	}
}

func TestIncrementalSync(t *testing.T) {
	tests := []struct {
		name          string
		syncToken     string
		expectedError bool
		errorType     ErrorType
	}{
		{
			name:          "valid token",
			syncToken:     "valid-token",
			expectedError: false,
		},
		{
			name:          "empty token",
			syncToken:     "",
			expectedError: true,
			errorType:     ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(207)
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>new-token</D:sync-token>
</D:multistatus>`))
			}))
			defer server.Close()

			client := NewClient("test", "test")
			client.baseURL = server.URL

			_, err := client.IncrementalSync(context.Background(), "/calendars/user/", tt.syncToken)

			if tt.expectedError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorType != 0 {
					if calErr, ok := err.(*CalDAVError); ok {
						if calErr.Type != tt.errorType {
							t.Errorf("Expected error type %v, got %v", tt.errorType, calErr.Type)
						}
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSyncAllCalendars(t *testing.T) {
	propfindCalls := 0
	syncCalled := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PROPFIND":
			propfindCalls++
			w.WriteHeader(207)

			switch propfindCalls {
			case 1:
				// FindCurrentUserPrincipal response
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/</D:href>
    <D:propstat>
      <D:prop>
        <D:current-user-principal>
          <D:href>/principal/</D:href>
        </D:current-user-principal>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			case 2:
				// FindCalendarHomeSet response
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/principal/</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-home-set>
          <D:href>/calendars/user/</D:href>
        </C:calendar-home-set>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			case 3:
				// FindCalendars response
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/user/calendar1/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Calendar 1</D:displayname>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/calendars/user/calendar2/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Calendar 2</D:displayname>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			}
		case "REPORT":
			syncCalled++
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>token-` + r.URL.Path + `</D:sync-token>
</D:multistatus>`))
		}
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	syncTokens := map[string]string{
		"/calendars/user/calendar1/": "old-token-1",
	}

	results, err := client.SyncAllCalendars(context.Background(), syncTokens)
	if err != nil {
		t.Fatalf("SyncAllCalendars failed: %v", err)
	}

	if propfindCalls != 3 {
		t.Errorf("Expected 3 PROPFIND calls, got %d", propfindCalls)
	}

	if syncCalled != 2 {
		t.Errorf("Expected 2 sync calls, got %d", syncCalled)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestBuildSyncCollectionXML(t *testing.T) {
	tests := []struct {
		name     string
		request  *SyncRequest
		contains []string
	}{
		{
			name: "initial sync",
			request: &SyncRequest{
				SyncToken:  "",
				SyncLevel:  1,
				Properties: []string{"getetag", "calendar-data"},
			},
			contains: []string{
				`<?xml version="1.0" encoding="UTF-8"?>`,
				`<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`,
				`<D:sync-token/>`,
				`<D:sync-level>1</D:sync-level>`,
				`<D:getetag/>`,
				`<C:calendar-data/>`,
				`</D:sync-collection>`,
			},
		},
		{
			name: "incremental sync with token",
			request: &SyncRequest{
				SyncToken:  "http://example.com/sync/1234",
				SyncLevel:  1,
				Properties: []string{"getetag"},
			},
			contains: []string{
				`<D:sync-token>http://example.com/sync/1234</D:sync-token>`,
				`<D:sync-level>1</D:sync-level>`,
				`<D:getetag/>`,
			},
		},
		{
			name: "sync with limit",
			request: &SyncRequest{
				SyncToken:  "",
				SyncLevel:  1,
				Limit:      100,
				Properties: []string{"displayname"},
			},
			contains: []string{
				`<D:limit><D:nresults>100</D:nresults></D:limit>`,
				`<D:displayname/>`,
			},
		},
		{
			name: "sync with special characters in token",
			request: &SyncRequest{
				SyncToken:  "token<>&\"'",
				SyncLevel:  1,
				Properties: []string{},
			},
			contains: []string{
				`<D:sync-token>token&lt;&gt;&amp;&#34;&#39;</D:sync-token>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml := buildSyncCollectionXML(tt.request)

			for _, expected := range tt.contains {
				if !strings.Contains(xml, expected) {
					t.Errorf("Expected XML to contain %s, but it doesn't. Full XML:\n%s", expected, xml)
				}
			}
		})
	}
}

func TestSyncAllCalendarsWithWorkers(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()

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

		if r.URL.Path == "/.well-known/caldav" {
			w.Header().Set("Location", "/principals/")
			w.WriteHeader(301)
			return
		}

		if r.Method == "PROPFIND" && r.URL.Path == "/principals/" {
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/principals/test/</D:href>
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
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/calendar1/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <D:displayname>Calendar 1</D:displayname>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/calendars/test/calendar2/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <D:displayname>Calendar 2</D:displayname>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/calendars/test/calendar3/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <D:displayname>Calendar 3</D:displayname>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			return
		}

		if r.Method == "REPORT" {
			calName := "unknown"
			if strings.Contains(r.URL.Path, "calendar1") {
				calName = "calendar1"
			} else if strings.Contains(r.URL.Path, "calendar2") {
				calName = "calendar2"
			} else if strings.Contains(r.URL.Path, "calendar3") {
				calName = "calendar3"
			}

			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>` + calName + `-token</D:sync-token>
  <D:response>
    <D:href>/calendars/test/` + calName + `/event.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag-` + calName + `"</D:getetag>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
			return
		}

		w.WriteHeader(404)
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	t.Run("parallel sync without existing tokens", func(t *testing.T) {
		results, err := client.SyncAllCalendarsWithWorkers(context.Background(), nil, 3)
		if err != nil {
			t.Fatalf("SyncAllCalendarsWithWorkers failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 sync results, got %d", len(results))
		}

		expectedCalendars := []string{
			"/calendars/test/calendar1/",
			"/calendars/test/calendar2/",
			"/calendars/test/calendar3/",
		}

		for _, calHref := range expectedCalendars {
			syncResp, exists := results[calHref]
			if !exists {
				t.Errorf("Missing sync result for calendar %s", calHref)
				continue
			}

			expectedToken := strings.Trim(calHref, "/")
			expectedToken = expectedToken[strings.LastIndex(expectedToken, "/")+1:] + "-token"
			if syncResp.SyncToken != expectedToken {
				t.Errorf("Calendar %s: expected token %s, got %s", calHref, expectedToken, syncResp.SyncToken)
			}
		}
	})

	t.Run("parallel sync with existing tokens", func(t *testing.T) {
		syncTokens := map[string]string{
			"/calendars/test/calendar1/": "existing-token-1",
			"/calendars/test/calendar2/": "existing-token-2",
		}

		results, err := client.SyncAllCalendarsWithWorkers(context.Background(), syncTokens, 2)
		if err != nil {
			t.Fatalf("SyncAllCalendarsWithWorkers failed: %v", err)
		}

		if len(results) < 2 {
			t.Errorf("Expected at least 2 sync results, got %d", len(results))
		}
	})

	t.Run("parallel sync with default workers", func(t *testing.T) {
		results, err := client.SyncAllCalendarsWithWorkers(context.Background(), nil, 0)
		if err != nil {
			t.Fatalf("SyncAllCalendarsWithWorkers failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 sync results, got %d", len(results))
		}
	})
}

func TestParseSyncResponse(t *testing.T) {
	tests := []struct {
		name          string
		xmlBody       string
		expectedToken string
		expectedCount int
		hasDeleted    bool
		expectedError bool
	}{
		{
			name: "response with new items",
			xmlBody: `<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>new-sync-token</D:sync-token>
  <D:response>
    <D:href>/event1.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag1"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
END:VCALENDAR</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`,
			expectedToken: "new-sync-token",
			expectedCount: 1,
			hasDeleted:    false,
		},
		{
			name: "response with deleted items",
			xmlBody: `<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:sync-token>delete-token</D:sync-token>
  <D:response>
    <D:href>/deleted.ics</D:href>
    <D:propstat>
      <D:prop/>
      <D:status>HTTP/1.1 404 Not Found</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`,
			expectedToken: "delete-token",
			expectedCount: 1,
			hasDeleted:    true,
		},
		{
			name:          "invalid XML",
			xmlBody:       `invalid xml`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseSyncResponse([]byte(tt.xmlBody))

			if tt.expectedError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if resp.SyncToken != tt.expectedToken {
				t.Errorf("Expected sync token %s, got %s", tt.expectedToken, resp.SyncToken)
			}

			if len(resp.Changes) != tt.expectedCount {
				t.Errorf("Expected %d changes, got %d", tt.expectedCount, len(resp.Changes))
			}

			if tt.hasDeleted && len(resp.Changes) > 0 {
				if !resp.Changes[0].Deleted {
					t.Error("Expected first change to be marked as deleted")
				}
			}
		})
	}
}

func TestSyncChangeType(t *testing.T) {
	tests := []struct {
		name         string
		change       SyncChange
		expectedType SyncChangeType
	}{
		{
			name: "deleted item",
			change: SyncChange{
				Deleted: true,
			},
			expectedType: SyncChangeTypeDeleted,
		},
		{
			name: "modified item",
			change: SyncChange{
				ETag:         "etag123",
				CalendarData: "VCALENDAR data",
			},
			expectedType: SyncChangeTypeModified,
		},
		{
			name: "new item",
			change: SyncChange{
				CalendarData: "VCALENDAR data",
			},
			expectedType: SyncChangeTypeNew,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.change.ChangeType(); got != tt.expectedType {
				t.Errorf("Expected change type %v, got %v", tt.expectedType, got)
			}
		})
	}
}

func TestSyncResponseHelpers(t *testing.T) {
	resp := &SyncResponse{
		SyncToken: "token",
		Changes: []SyncChange{
			{Href: "/new.ics", CalendarData: "data"},
			{Href: "/modified.ics", ETag: "etag", CalendarData: "data"},
			{Href: "/deleted.ics", Deleted: true},
		},
	}

	t.Run("GetNewItems", func(t *testing.T) {
		items := resp.GetNewItems()
		if len(items) != 1 {
			t.Errorf("Expected 1 new item, got %d", len(items))
		}
		if items[0].Href != "/new.ics" {
			t.Errorf("Expected new item href /new.ics, got %s", items[0].Href)
		}
	})

	t.Run("GetModifiedItems", func(t *testing.T) {
		items := resp.GetModifiedItems()
		if len(items) != 1 {
			t.Errorf("Expected 1 modified item, got %d", len(items))
		}
		if items[0].Href != "/modified.ics" {
			t.Errorf("Expected modified item href /modified.ics, got %s", items[0].Href)
		}
	})

	t.Run("GetDeletedItems", func(t *testing.T) {
		items := resp.GetDeletedItems()
		if len(items) != 1 {
			t.Errorf("Expected 1 deleted item, got %d", len(items))
		}
		if items[0].Href != "/deleted.ics" {
			t.Errorf("Expected deleted item href /deleted.ics, got %s", items[0].Href)
		}
	})

	t.Run("HasChanges", func(t *testing.T) {
		if !resp.HasChanges() {
			t.Error("Expected HasChanges to return true")
		}

		emptyResp := &SyncResponse{Changes: []SyncChange{}}
		if emptyResp.HasChanges() {
			t.Error("Expected HasChanges to return false for empty response")
		}
	})
}

func TestIntToString(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{-5, "0"},
		{1, "1"},
		{100, "100"},
		{12345, "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := intToString(tt.input); got != tt.expected {
				t.Errorf("intToString(%d) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestETagCache(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.Method == "HEAD" && r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "test content")
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	ctx := context.Background()

	entry1, err := client.GetWithETag(ctx, "/test.ics")
	if err != nil {
		t.Fatal(err)
	}
	if string(entry1.Content) != "test content" {
		t.Errorf("Expected 'test content', got %s", entry1.Content)
	}
	if entry1.ETag != `"v1"` {
		t.Errorf("Expected ETag 'v1', got %s", entry1.ETag)
	}

	entry2, err := client.GetWithETag(ctx, "/test.ics")
	if err != nil {
		t.Fatal(err)
	}
	if string(entry2.Content) != "test content" {
		t.Errorf("Cached content mismatch")
	}

	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("Expected 2 requests (GET + HEAD), got %d", requestCount)
	}
}

func TestCacheInvalidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v2"`)
		_, _ = fmt.Fprint(w, "updated content")
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	ctx := context.Background()

	_, err := client.GetWithETag(ctx, "/test.ics")
	if err != nil {
		t.Fatal(err)
	}

	client.InvalidateCache("/test.ics")

	entry, err := client.GetWithETag(ctx, "/test.ics")
	if err != nil {
		t.Fatal(err)
	}
	if string(entry.Content) != "updated content" {
		t.Errorf("Expected fresh content after invalidation")
	}
}

func TestPreferHeader(t *testing.T) {
	var capturedPrefer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPrefer = r.Header.Get("Prefer")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	maxResults := 10
	wait := 5 * time.Second
	client.SetPreferDefaults(&PreferHeader{
		ReturnMinimal:  true,
		Wait:           &wait,
		MaxResults:     &maxResults,
		HandlingStrict: true,
		RespondAsync:   true,
		DepthNoroot:    true,
	})

	ctx := context.Background()
	_, _ = client.GetWithETag(ctx, "/test")

	expected := "return=minimal, wait=5, handling=strict, respond-async, depth-noroot, max-results=10"
	if capturedPrefer != expected {
		t.Errorf("Expected Prefer header '%s', got '%s'", expected, capturedPrefer)
	}
}

func TestBatchOperations(t *testing.T) {
	var requestPaths []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestPaths = append(requestPaths, r.URL.Path)
		mu.Unlock()

		switch r.Method {
		case "PUT":
			w.Header().Set("ETag", `"created"`)
			w.WriteHeader(http.StatusCreated)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("ETag", `"v1"`)
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "content")
		}
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)
	client.SetBatchSize(2)

	operations := []BatchOperation{
		{Method: "GET", Path: "/cal1.ics"},
		{Method: "PUT", Path: "/cal2.ics", Body: []byte("new event")},
		{Method: "DELETE", Path: "/cal3.ics"},
		{Method: "GET", Path: "/cal4.ics", Headers: map[string]string{"Accept": "text/calendar"}},
	}

	ctx := context.Background()
	results, err := client.BatchExecute(ctx, operations)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

	if results[0].StatusCode != http.StatusOK {
		t.Errorf("Operation 0: expected 200, got %d", results[0].StatusCode)
	}
	if results[1].StatusCode != http.StatusCreated {
		t.Errorf("Operation 1: expected 201, got %d", results[1].StatusCode)
	}
	if results[2].StatusCode != http.StatusNoContent {
		t.Errorf("Operation 2: expected 204, got %d", results[2].StatusCode)
	}
	if results[3].StatusCode != http.StatusOK {
		t.Errorf("Operation 3: expected 200, got %d", results[3].StatusCode)
	}

	if len(requestPaths) != 4 {
		t.Errorf("Expected 4 requests, got %d", len(requestPaths))
	}
}

func TestDeltaSync(t *testing.T) {
	syncCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		syncCount++

		if syncCount == 1 {
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>/calendars/user/cal1.ics</href>
    <propstat>
      <prop>
        <getetag>"etag1"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
  <response>
    <href>/calendars/user/cal2.ics</href>
    <propstat>
      <prop>
        <getetag>"etag2"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
  <sync-token>sync-token-v1</sync-token>
</multistatus>`)
		} else {
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>/calendars/user/cal1.ics</href>
    <status>HTTP/1.1 404 Not Found</status>
  </response>
  <response>
    <href>/calendars/user/cal3.ics</href>
    <propstat>
      <prop>
        <getetag>"etag3"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
  <sync-token>sync-token-v2</sync-token>
</multistatus>`)
		}
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	ctx := context.Background()

	state1, err := client.DeltaSync(ctx, "/calendars/user/")
	if err != nil {
		t.Fatal(err)
	}

	if state1.SyncToken != "sync-token-v1" {
		t.Errorf("Expected sync token 'sync-token-v1', got '%s'", state1.SyncToken)
	}
	if len(state1.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(state1.Resources))
	}

	state2, err := client.DeltaSync(ctx, "/calendars/user/")
	if err != nil {
		t.Fatal(err)
	}

	if state2.SyncToken != "sync-token-v2" {
		t.Errorf("Expected sync token 'sync-token-v2', got '%s'", state2.SyncToken)
	}
	if len(state2.Resources) != 2 {
		t.Errorf("Expected 2 resources after delta, got %d", len(state2.Resources))
	}
	if _, exists := state2.Resources["/calendars/user/cal1.ics"]; exists {
		t.Error("Deleted resource should not exist")
	}
	if _, exists := state2.Resources["/calendars/user/cal3.ics"]; !exists {
		t.Error("New resource should exist")
	}
	if len(state2.PendingDeletes) != 1 {
		t.Errorf("Expected 1 pending delete, got %d", len(state2.PendingDeletes))
	}
}

func TestCacheCompaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, r.URL.Path))
		_, _ = fmt.Fprintf(w, "content for %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)
	client.SetCacheMaxAge(100 * time.Millisecond)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		path := fmt.Sprintf("/test%d.ics", i)
		_, err := client.GetWithETag(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
	}

	entries, size := client.GetCacheStats()
	if entries != 5 {
		t.Errorf("Expected 5 cache entries, got %d", entries)
	}
	if size == 0 {
		t.Error("Cache size should not be zero")
	}

	time.Sleep(150 * time.Millisecond)
	client.CompactCache()

	entries, _ = client.GetCacheStats()
	if entries != 0 {
		t.Errorf("Expected 0 entries after compaction, got %d", entries)
	}
}

func TestPreloadCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", fmt.Sprintf(`"etag-%s"`, r.URL.Path))
		_, _ = fmt.Fprintf(w, "content for %s", r.URL.Path)
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	paths := []string{"/cal1.ics", "/cal2.ics", "/cal3.ics"}
	ctx := context.Background()

	err := client.PreloadCache(ctx, paths)
	if err != nil {
		t.Fatal(err)
	}

	entries, size := client.GetCacheStats()
	if entries != 3 {
		t.Errorf("Expected 3 preloaded entries, got %d", entries)
	}
	if size == 0 {
		t.Error("Preloaded cache should have size > 0")
	}

	for _, path := range paths {
		if entry, exists := client.etagCache.entries[path]; !exists {
			t.Errorf("Path %s not in cache", path)
		} else if !strings.Contains(string(entry.Content), path) {
			t.Errorf("Cache content mismatch for %s", path)
		}
	}
}

func TestBatchWithConditionalRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"existing"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.Header.Get("If-Match") == `"wrong"` {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}

		w.Header().Set("ETag", `"new"`)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "updated")
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	operations := []BatchOperation{
		{Method: "GET", Path: "/test1", IfNoneMatch: `"existing"`},
		{Method: "PUT", Path: "/test2", IfMatch: `"wrong"`, Body: []byte("data")},
		{Method: "GET", Path: "/test3"},
	}

	ctx := context.Background()
	results, err := client.BatchExecute(ctx, operations)
	if err != nil {
		t.Fatal(err)
	}

	if results[0].StatusCode != http.StatusNotModified {
		t.Errorf("Expected 304 for If-None-Match, got %d", results[0].StatusCode)
	}
	if results[1].StatusCode != http.StatusPreconditionFailed {
		t.Errorf("Expected 412 for If-Match failure, got %d", results[1].StatusCode)
	}
	if results[2].StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for normal request, got %d", results[2].StatusCode)
	}
}

func TestDeltaSyncWithNoChanges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <sync-token>same-token</sync-token>
</multistatus>`)
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	ctx := context.Background()
	state, err := client.DeltaSync(ctx, "/calendars/")
	if err != nil {
		t.Fatal(err)
	}

	if state.SyncToken != "same-token" {
		t.Errorf("Expected sync token 'same-token', got '%s'", state.SyncToken)
	}
	if len(state.Resources) != 0 {
		t.Errorf("Expected no resources for empty sync, got %d", len(state.Resources))
	}
}

func TestCacheMaxAgeConfiguration(t *testing.T) {
	client := NewClient("user", "pass")

	if client.etagCache.maxAge != 15*time.Minute {
		t.Errorf("Default cache max age should be 15 minutes")
	}

	client.SetCacheMaxAge(5 * time.Minute)
	if client.etagCache.maxAge != 5*time.Minute {
		t.Errorf("Cache max age not updated correctly")
	}
}

func TestBatchSizeConfiguration(t *testing.T) {
	client := NewClient("user", "pass")

	if client.batchSize != 50 {
		t.Errorf("Default batch size should be 50")
	}

	client.SetBatchSize(100)
	if client.batchSize != 100 {
		t.Errorf("Batch size not updated correctly")
	}

	client.SetBatchSize(0)
	if client.batchSize != 100 {
		t.Errorf("Batch size should not change for invalid value")
	}

	client.SetBatchSize(-5)
	if client.batchSize != 100 {
		t.Errorf("Batch size should not change for negative value")
	}
}

func TestClearCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test"`)
		_, _ = fmt.Fprint(w, "content")
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := client.GetWithETag(ctx, fmt.Sprintf("/test%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	entries, _ := client.GetCacheStats()
	if entries != 3 {
		t.Errorf("Expected 3 cache entries, got %d", entries)
	}

	client.ClearCache()

	entries, size := client.GetCacheStats()
	if entries != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", entries)
	}
	if size != 0 {
		t.Errorf("Expected 0 size after clear, got %d", size)
	}
}

func TestGetDeltaResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>/cal/event1.ics</href>
    <propstat>
      <prop>
        <getetag>"etag1"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
  <sync-token>token1</sync-token>
</multistatus>`)
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.SetBaseURL(server.URL)

	_, err := client.GetDeltaResources("/calendars/")
	if err == nil {
		t.Error("Expected error for non-existent sync state")
	}

	ctx := context.Background()
	_, err = client.DeltaSync(ctx, "/calendars/")
	if err != nil {
		t.Fatal(err)
	}

	resources, err := client.GetDeltaResources("/calendars/")
	if err != nil {
		t.Fatal(err)
	}

	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}

	if resource, exists := resources["/cal/event1.ics"]; !exists {
		t.Error("Expected resource not found")
	} else if resource.ETag != `"etag1"` {
		t.Errorf("Expected ETag 'etag1', got '%s'", resource.ETag)
	}
}
