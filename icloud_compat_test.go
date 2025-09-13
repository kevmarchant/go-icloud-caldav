package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDetectServerType(t *testing.T) {
	tests := []struct {
		name         string
		serverHeader string
		davHeader    string
		baseURL      string
		expectedType ServerType
	}{
		{
			name:         "iCloud server by URL",
			serverHeader: "AppleHttpServer/2.0",
			davHeader:    "1, 2, calendar-access, calendar-managed-attachments",
			baseURL:      "https://caldav.icloud.com",
			expectedType: ServerTypeICloud,
		},
		{
			name:         "iCloud server by header",
			serverHeader: "iCloud/1.0",
			davHeader:    "1, 2, calendar-access",
			baseURL:      "https://example.com",
			expectedType: ServerTypeICloud,
		},
		{
			name:         "Google server",
			serverHeader: "Google-Calendar-Server/1.0",
			davHeader:    "1, 2, calendar-access",
			baseURL:      "https://caldav.google.com",
			expectedType: ServerTypeGoogle,
		},
		{
			name:         "Nextcloud server",
			serverHeader: "Nextcloud/25.0.0",
			davHeader:    "1, 2, calendar-access, calendar-schedule",
			baseURL:      "https://cloud.example.com",
			expectedType: ServerTypeNextcloud,
		},
		{
			name:         "Generic server",
			serverHeader: "Apache/2.4",
			davHeader:    "1, 2, calendar-access",
			baseURL:      "https://caldav.example.com",
			expectedType: ServerTypeGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("DAV", tt.davHeader)
				w.Header().Set("Server", tt.serverHeader)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := &CalDAVClient{
				baseURL:    tt.baseURL,
				httpClient: &http.Client{},
				authHeader: "Basic dGVzdDp0ZXN0",
			}

			if tt.baseURL == "https://caldav.icloud.com" ||
				tt.baseURL == "https://caldav.google.com" ||
				tt.baseURL == "https://cloud.example.com" ||
				tt.baseURL == "https://caldav.example.com" ||
				tt.baseURL == "https://example.com" {
				client.baseURL = server.URL
			}

			compat, err := client.DetectServerType(context.Background())
			if err != nil {
				t.Fatalf("DetectServerType failed: %v", err)
			}

			if compat.Type != tt.expectedType {
				t.Errorf("Expected server type %s, got %s", tt.expectedType, compat.Type)
			}
		})
	}
}

func TestICloudCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("DAV", "1, 2, calendar-access, calendar-managed-attachments, calendarserver-sharing")
		w.Header().Set("Server", "AppleHttpServer/2.0")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authHeader: "Basic dGVzdDp0ZXN0",
	}

	compat, err := client.DetectServerType(context.Background())
	if err != nil {
		t.Fatalf("DetectServerType failed: %v", err)
	}

	expectedCapabilities := map[ServerCapability]bool{
		CapCalendarAccess:        true,
		CapCalendarManagedAttach: true,
		CapCalendarServerSharing: true,
		CapCalendarSchedule:      false,
		CapCalendarAvailability:  false,
		CapVJournal:              false,
		CapVResource:             false,
		CapVAvailability:         false,
	}

	for cap, expected := range expectedCapabilities {
		if compat.Capabilities[cap] != expected {
			t.Errorf("Capability %s: expected %v, got %v", cap, expected, compat.Capabilities[cap])
		}
	}
}

func TestIsICloudServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("DAV", "1, 2, calendar-access")
		w.Header().Set("Server", "iCloud/1.0")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authHeader: "Basic dGVzdDp0ZXN0",
	}

	isICloud, err := client.IsICloudServer(context.Background())
	if err != nil {
		t.Fatalf("IsICloudServer failed: %v", err)
	}

	if !isICloud {
		t.Error("Expected server to be identified as iCloud")
	}
}

func TestSupportsFeature(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("DAV", "1, 2, calendar-access, calendar-managed-attachments")
		w.Header().Set("Server", "iCloud/1.0")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authHeader: "Basic dGVzdDp0ZXN0",
	}

	tests := []struct {
		capability ServerCapability
		expected   bool
	}{
		{CapCalendarAccess, true},
		{CapCalendarManagedAttach, true},
		{CapCalendarSchedule, false},
		{CapVJournal, false},
	}

	for _, tt := range tests {
		supported, err := client.SupportsFeature(context.Background(), tt.capability)
		if err != nil {
			t.Fatalf("SupportsFeature failed for %s: %v", tt.capability, err)
		}

		if supported != tt.expected {
			t.Errorf("Capability %s: expected %v, got %v", tt.capability, tt.expected, supported)
		}
	}
}

func TestIsFeatureSupportedByICloud(t *testing.T) {
	tests := []struct {
		capability ServerCapability
		expected   bool
	}{
		{CapCalendarAccess, true},
		{CapCalendarProxy, true},
		{CapCalendarQueryExtended, true},
		{CapCalendarManagedAttach, true},
		{CapCalendarNoInstance, true},
		{CapCalendarServerSharing, true},
		{CapCalendarServerSubscribed, true},
		{CapCalendarServerHomeSync, true},
		{CapCalendarServerComments, true},
		{CapCalendarSchedule, false},
		{CapCalendarAutoSchedule, false},
		{CapCalendarAvailability, false},
		{CapInboxAvailability, false},
		{CapVJournal, false},
		{CapVResource, false},
		{CapVAvailability, false},
	}

	for _, tt := range tests {
		result := IsFeatureSupportedByICloud(tt.capability)
		if result != tt.expected {
			t.Errorf("Capability %s: expected %v, got %v", tt.capability, tt.expected, result)
		}
	}
}

func TestGetICloudSpecificHeaders(t *testing.T) {
	headers := GetICloudSpecificHeaders()

	expectedHeaders := map[string]string{
		"X-Apple-Calendar-User-Agent": "go-icloud-caldav/1.0",
		"Accept":                      "text/calendar, text/xml, application/xml",
		"Accept-Language":             "en-US,en;q=0.9",
	}

	for key, expected := range expectedHeaders {
		if headers[key] != expected {
			t.Errorf("Header %s: expected %s, got %s", key, expected, headers[key])
		}
	}
}

func TestConfigureForICloud(t *testing.T) {
	client := &CalDAVClient{
		baseURL:    "https://caldav.icloud.com",
		httpClient: &http.Client{},
		authHeader: "Basic dGVzdDp0ZXN0",
	}

	client.ConfigureForICloud()

	if !strings.HasSuffix(client.baseURL, "/") {
		t.Error("Expected baseURL to end with /")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", client.httpClient.Timeout)
	}
}
