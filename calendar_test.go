package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindCurrentUserPrincipal(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected method PROPFIND, got %s", r.Method)
		}
		if r.URL.Path != "/" {
			t.Errorf("expected path /, got %s", r.URL.Path)
		}
		if r.Header.Get("Depth") != "0" {
			t.Errorf("expected Depth 0, got %s", r.Header.Get("Depth"))
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	principal, err := client.FindCurrentUserPrincipal(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if principal != "/123456/principal/" {
		t.Errorf("expected principal /123456/principal/, got %s", principal)
	}
}

func TestFindCalendarHomeSet(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected method PROPFIND, got %s", r.Method)
		}
		if r.URL.Path != "/123456/principal/" {
			t.Errorf("expected path /123456/principal/, got %s", r.URL.Path)
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	homeSet, err := client.FindCalendarHomeSet(context.Background(), "/123456/principal/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if homeSet != "/123456/calendars/" {
		t.Errorf("expected calendar home set /123456/calendars/, got %s", homeSet)
	}
}

func TestFindCalendars(t *testing.T) {
	responseXML := `<?xml version="1.0" encoding="utf-8"?>
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
</D:multistatus>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected method PROPFIND, got %s", r.Method)
		}
		if r.URL.Path != "/123456/calendars/" {
			t.Errorf("expected path /123456/calendars/, got %s", r.URL.Path)
		}
		if r.Header.Get("Depth") != "1" {
			t.Errorf("expected Depth 1, got %s", r.Header.Get("Depth"))
		}

		w.WriteHeader(207)
		_, _ = w.Write([]byte(responseXML))
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	calendars, err := client.FindCalendars(context.Background(), "/123456/calendars/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calendars) != 2 {
		t.Fatalf("expected 2 calendars, got %d", len(calendars))
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

	workCal := calendars[1]
	if workCal.DisplayName != "Work" {
		t.Errorf("expected display name 'Work', got %s", workCal.DisplayName)
	}
}

func TestDiscoverCalendars(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch callCount {
		case 1:
			if r.URL.Path != "/" {
				t.Errorf("first call: expected path /, got %s", r.URL.Path)
			}
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
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
</D:multistatus>`))

		case 2:
			if r.URL.Path != "/123456/principal/" {
				t.Errorf("second call: expected path /123456/principal/, got %s", r.URL.Path)
			}
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
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
</D:multistatus>`))

		case 3:
			if r.URL.Path != "/123456/calendars/" {
				t.Errorf("third call: expected path /123456/calendars/, got %s", r.URL.Path)
			}
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	<D:response>
		<D:href>/123456/calendars/home/</D:href>
		<D:propstat>
			<D:status>HTTP/1.1 200 OK</D:status>
			<D:prop>
				<D:displayname>Home</D:displayname>
				<D:resourcetype>
					<D:collection/>
					<C:calendar/>
				</D:resourcetype>
			</D:prop>
		</D:propstat>
	</D:response>
</D:multistatus>`))
		}
	}))
	defer server.Close()

	client := NewClient("testuser", "testpass")
	client.baseURL = server.URL

	calendars, err := client.DiscoverCalendars(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calendars) != 1 {
		t.Fatalf("expected 1 calendar, got %d", len(calendars))
	}

	if calendars[0].DisplayName != "Home" {
		t.Errorf("expected display name 'Home', got %s", calendars[0].DisplayName)
	}

	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}
