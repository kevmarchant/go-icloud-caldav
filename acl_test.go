package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParsePrivilegeSet(t *testing.T) {
	privileges := []string{"read", "write", "write-properties", "calendar-access"}
	privSet := ParsePrivilegeSet(privileges)

	if !privSet.Read {
		t.Error("expected Read to be true")
	}
	if !privSet.Write {
		t.Error("expected Write to be true")
	}
	if !privSet.WriteProperties {
		t.Error("expected WriteProperties to be true")
	}
	if !privSet.CalendarAccess {
		t.Error("expected CalendarAccess to be true")
	}
	if privSet.WriteContent {
		t.Error("expected WriteContent to be false")
	}
}

func TestPrivilegeSetToStringSlice(t *testing.T) {
	privSet := PrivilegeSet{
		Read:           true,
		Write:          true,
		CalendarAccess: true,
		ReadFreeBusy:   true,
	}

	privileges := privSet.ToStringSlice()
	expected := []string{"read", "write", "calendar-access", "read-free-busy"}

	if len(privileges) != len(expected) {
		t.Errorf("expected %d privileges, got %d", len(expected), len(privileges))
	}

	for i, exp := range expected {
		if i >= len(privileges) || privileges[i] != exp {
			t.Errorf("expected privilege[%d] to be '%s', got '%s'", i, exp, privileges[i])
		}
	}
}

func TestFindPrincipal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected PROPFIND request, got %s", r.Method)
		}

		if r.Header.Get("Depth") != "0" {
			t.Errorf("expected Depth: 0 header, got %s", r.Header.Get("Depth"))
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
		<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
			<d:response>
				<d:href>/principals/testuser/</d:href>
				<d:propstat>
					<d:status>HTTP/1.1 200 OK</d:status>
					<d:prop>
						<d:displayname>Test User</d:displayname>
						<d:resourcetype>
							<d:principal/>
						</d:resourcetype>
						<c:calendar-home-set>
							<d:href>/testuser/calendars/</d:href>
						</c:calendar-home-set>
					</d:prop>
				</d:propstat>
			</d:response>
		</d:multistatus>`))
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		httpClient: server.Client(),
		logger:     &noopLogger{},
	}

	principal, err := client.FindPrincipal(context.Background(), "/principals/testuser/")
	if err != nil {
		t.Fatalf("FindPrincipal failed: %v", err)
	}

	if principal.Href != "/principals/testuser/" {
		t.Errorf("expected href '/principals/testuser/', got %s", principal.Href)
	}

	if principal.DisplayName != "Test User" {
		t.Errorf("expected display name 'Test User', got %s", principal.DisplayName)
	}

	if principal.Type != "user" {
		t.Errorf("expected type 'user', got %s", principal.Type)
	}
}

func TestGetACL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Errorf("expected PROPFIND request, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
		<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
			<d:response>
				<d:href>/testuser/calendars/home/</d:href>
				<d:propstat>
					<d:status>HTTP/1.1 200 OK</d:status>
					<d:prop>
						<d:current-user-privilege-set>
							<d:privilege>
								<d:read/>
							</d:privilege>
							<d:privilege>
								<d:write/>
							</d:privilege>
							<d:privilege>
								<c:calendar-access/>
							</d:privilege>
						</d:current-user-privilege-set>
					</d:prop>
				</d:propstat>
			</d:response>
		</d:multistatus>`))
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		httpClient: server.Client(),
		logger:     &noopLogger{},
	}

	acl, err := client.GetACL(context.Background(), "/testuser/calendars/home/")
	if err != nil {
		t.Fatalf("GetACL failed: %v", err)
	}

	if len(acl.ACEs) != 1 {
		t.Errorf("expected 1 ACE, got %d", len(acl.ACEs))
	}

	ace := acl.ACEs[0]
	expectedPrivileges := []string{"read", "write", "calendar-access"}

	if len(ace.Grant) != len(expectedPrivileges) {
		t.Errorf("expected %d privileges, got %d", len(expectedPrivileges), len(ace.Grant))
	}

	for i, expected := range expectedPrivileges {
		if i >= len(ace.Grant) || ace.Grant[i] != expected {
			t.Errorf("expected privilege[%d] to be '%s', got '%s'", i, expected, ace.Grant[i])
		}
	}
}

func TestCheckPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
		<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
			<d:response>
				<d:href>/testuser/calendars/home/</d:href>
				<d:propstat>
					<d:status>HTTP/1.1 200 OK</d:status>
					<d:prop>
						<d:current-user-privilege-set>
							<d:privilege>
								<d:read/>
							</d:privilege>
							<d:privilege>
								<d:write/>
							</d:privilege>
						</d:current-user-privilege-set>
					</d:prop>
				</d:propstat>
			</d:response>
		</d:multistatus>`))
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		httpClient: server.Client(),
		logger:     &noopLogger{},
	}

	// Test read permission (should be true)
	hasRead, err := client.CheckPermission(context.Background(), "/testuser/calendars/home/", "read")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !hasRead {
		t.Error("expected read permission to be true")
	}

	// Test write permission (should be true)
	hasWrite, err := client.CheckPermission(context.Background(), "/testuser/calendars/home/", "write")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !hasWrite {
		t.Error("expected write permission to be true")
	}

	// Test delete permission (should be false)
	hasDelete, err := client.CheckPermission(context.Background(), "/testuser/calendars/home/", "unbind")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if hasDelete {
		t.Error("expected delete permission to be false")
	}
}

func TestHasReadAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
		<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
			<d:response>
				<d:href>/testuser/calendars/home/</d:href>
				<d:propstat>
					<d:status>HTTP/1.1 200 OK</d:status>
					<d:prop>
						<d:current-user-privilege-set>
							<d:privilege>
								<d:read/>
							</d:privilege>
						</d:current-user-privilege-set>
					</d:prop>
				</d:propstat>
			</d:response>
		</d:multistatus>`))
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		httpClient: server.Client(),
		logger:     &noopLogger{},
	}

	hasAccess, err := client.HasReadAccess(context.Background(), "/testuser/calendars/home/")
	if err != nil {
		t.Fatalf("HasReadAccess failed: %v", err)
	}
	if !hasAccess {
		t.Error("expected read access to be true")
	}
}

func TestHasWriteAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
		<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
			<d:response>
				<d:href>/testuser/calendars/home/</d:href>
				<d:propstat>
					<d:status>HTTP/1.1 200 OK</d:status>
					<d:prop>
						<d:current-user-privilege-set>
							<d:privilege>
								<d:read/>
							</d:privilege>
						</d:current-user-privilege-set>
					</d:prop>
				</d:propstat>
			</d:response>
		</d:multistatus>`))
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		username:   "testuser",
		password:   "testpass",
		httpClient: server.Client(),
		logger:     &noopLogger{},
	}

	hasAccess, err := client.HasWriteAccess(context.Background(), "/testuser/calendars/home/")
	if err != nil {
		t.Fatalf("HasWriteAccess failed: %v", err)
	}
	// Should be false since we only granted read permission
	if hasAccess {
		t.Error("expected write access to be false")
	}
}

func TestBuildPropfindXMLWithACLProperties(t *testing.T) {
	props := []string{"acl", "supported-privilege-set", "current-user-privilege-set"}
	xml, err := buildPropfindXML(props)
	if err != nil {
		t.Fatalf("buildPropfindXML failed: %v", err)
	}

	xmlStr := string(xml)

	if !strings.Contains(xmlStr, "<D:acl/>") {
		t.Error("expected XML to contain <D:acl/>")
	}
	if !strings.Contains(xmlStr, "<D:supported-privilege-set/>") {
		t.Error("expected XML to contain <D:supported-privilege-set/>")
	}
	if !strings.Contains(xmlStr, "<D:current-user-privilege-set/>") {
		t.Error("expected XML to contain <D:current-user-privilege-set/>")
	}
}
