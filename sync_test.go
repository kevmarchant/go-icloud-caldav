package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
