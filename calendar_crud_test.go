package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateCalendar(t *testing.T) {
	tests := []struct {
		name           string
		calendar       *Calendar
		homeSetPath    string
		serverResponse int
		expectError    bool
		expectedError  error
	}{
		{
			name: "successful_creation",
			calendar: &Calendar{
				DisplayName: "Test Calendar",
				Description: "My test calendar",
				Color:       "#FF0000FF",
			},
			homeSetPath:    "/user/calendars",
			serverResponse: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "calendar_already_exists",
			calendar: &Calendar{
				DisplayName: "Existing Calendar",
			},
			homeSetPath:    "/user/calendars",
			serverResponse: http.StatusConflict,
			expectError:    true,
			expectedError:  ErrCalendarAlreadyExists,
		},
		{
			name: "unauthorized",
			calendar: &Calendar{
				DisplayName: "Test Calendar",
			},
			homeSetPath:    "/user/calendars",
			serverResponse: http.StatusUnauthorized,
			expectError:    true,
			expectedError:  ErrUnauthorized,
		},
		{
			name: "forbidden",
			calendar: &Calendar{
				DisplayName: "Test Calendar",
			},
			homeSetPath:    "/user/calendars",
			serverResponse: http.StatusForbidden,
			expectError:    true,
			expectedError:  ErrForbidden,
		},
		{
			name: "missing_display_name",
			calendar: &Calendar{
				Description: "Missing name",
			},
			homeSetPath: "/user/calendars",
			expectError: true,
		},
		{
			name: "with_supported_components",
			calendar: &Calendar{
				DisplayName:         "Events and Tasks",
				SupportedComponents: []string{"VEVENT", "VTODO"},
			},
			homeSetPath:    "/user/calendars",
			serverResponse: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "with_timezone",
			calendar: &Calendar{
				DisplayName:      "Calendar with TZ",
				CalendarTimeZone: "Europe/London",
			},
			homeSetPath:    "/user/calendars",
			serverResponse: http.StatusCreated,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "MKCALENDAR" {
					t.Errorf("Expected MKCALENDAR method, got %s", r.Method)
				}

				body := make([]byte, r.ContentLength)
				_, _ = r.Body.Read(body)
				capturedBody = string(body)

				if tt.serverResponse != 0 {
					w.WriteHeader(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := &CalDAVClient{
				baseURL:    server.URL,
				authHeader: "Basic dGVzdDp0ZXN0",
				httpClient: &http.Client{},
				logger:     &testLogger{},
			}

			err := client.CreateCalendar(tt.homeSetPath, tt.calendar)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedError != nil && err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.calendar.DisplayName != "" && !strings.Contains(capturedBody, tt.calendar.DisplayName) {
					t.Errorf("Request body should contain display name")
				}

				if tt.calendar.Description != "" && !strings.Contains(capturedBody, tt.calendar.Description) {
					t.Errorf("Request body should contain description")
				}

				if tt.calendar.Color != "" && !strings.Contains(capturedBody, tt.calendar.Color) {
					t.Errorf("Request body should contain color")
				}

				if len(tt.calendar.SupportedComponents) > 0 {
					for _, comp := range tt.calendar.SupportedComponents {
						if !strings.Contains(capturedBody, comp) {
							t.Errorf("Request body should contain component %s", comp)
						}
					}
				}
			}
		})
	}
}

func TestUpdateCalendar(t *testing.T) {
	displayName := "Updated Calendar"
	description := "Updated description"
	color := "#00FF00FF"
	timezone := "America/New_York"

	tests := []struct {
		name           string
		calendarPath   string
		updates        *CalendarPropertyUpdate
		serverResponse int
		expectError    bool
		expectedError  error
	}{
		{
			name:         "update_display_name",
			calendarPath: "/user/calendars/test/",
			updates: &CalendarPropertyUpdate{
				DisplayName: &displayName,
			},
			serverResponse: http.StatusOK,
			expectError:    false,
		},
		{
			name:         "update_all_properties",
			calendarPath: "/user/calendars/test/",
			updates: &CalendarPropertyUpdate{
				DisplayName:      &displayName,
				Description:      &description,
				Color:            &color,
				CalendarTimeZone: &timezone,
			},
			serverResponse: http.StatusMultiStatus,
			expectError:    false,
		},
		{
			name:         "calendar_not_found",
			calendarPath: "/user/calendars/nonexistent/",
			updates: &CalendarPropertyUpdate{
				DisplayName: &displayName,
			},
			serverResponse: http.StatusNotFound,
			expectError:    true,
			expectedError:  ErrCalendarNotFound,
		},
		{
			name:         "forbidden",
			calendarPath: "/user/calendars/test/",
			updates: &CalendarPropertyUpdate{
				DisplayName: &displayName,
			},
			serverResponse: http.StatusForbidden,
			expectError:    true,
			expectedError:  ErrForbidden,
		},
		{
			name:         "no_updates",
			calendarPath: "/user/calendars/test/",
			updates:      &CalendarPropertyUpdate{},
			expectError:  true,
		},
		{
			name:         "nil_updates",
			calendarPath: "/user/calendars/test/",
			updates:      nil,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PROPPATCH" {
					t.Errorf("Expected PROPPATCH method, got %s", r.Method)
				}

				body := make([]byte, r.ContentLength)
				_, _ = r.Body.Read(body)
				capturedBody = string(body)

				if tt.serverResponse != 0 {
					w.WriteHeader(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := &CalDAVClient{
				baseURL:    server.URL,
				authHeader: "Basic dGVzdDp0ZXN0",
				httpClient: &http.Client{},
				logger:     &testLogger{},
			}

			err := client.UpdateCalendar(tt.calendarPath, tt.updates)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedError != nil && err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if tt.updates.DisplayName != nil && !strings.Contains(capturedBody, *tt.updates.DisplayName) {
					t.Errorf("Request body should contain display name")
				}

				if tt.updates.Description != nil && !strings.Contains(capturedBody, *tt.updates.Description) {
					t.Errorf("Request body should contain description")
				}

				if tt.updates.Color != nil && !strings.Contains(capturedBody, *tt.updates.Color) {
					t.Errorf("Request body should contain color")
				}

				if tt.updates.CalendarTimeZone != nil && !strings.Contains(capturedBody, *tt.updates.CalendarTimeZone) {
					t.Errorf("Request body should contain timezone")
				}
			}
		})
	}
}

func TestDeleteCalendar(t *testing.T) {
	tests := []struct {
		name           string
		calendarPath   string
		serverResponse int
		expectError    bool
		expectedError  error
	}{
		{
			name:           "successful_deletion",
			calendarPath:   "/user/calendars/test/",
			serverResponse: http.StatusNoContent,
			expectError:    false,
		},
		{
			name:           "already_deleted",
			calendarPath:   "/user/calendars/test/",
			serverResponse: http.StatusNotFound,
			expectError:    false,
		},
		{
			name:           "forbidden",
			calendarPath:   "/user/calendars/test/",
			serverResponse: http.StatusForbidden,
			expectError:    true,
			expectedError:  ErrForbidden,
		},
		{
			name:           "unauthorized",
			calendarPath:   "/user/calendars/test/",
			serverResponse: http.StatusUnauthorized,
			expectError:    true,
			expectedError:  ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("Expected DELETE method, got %s", r.Method)
				}

				if tt.serverResponse != 0 {
					w.WriteHeader(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := &CalDAVClient{
				baseURL:    server.URL,
				authHeader: "Basic dGVzdDp0ZXN0",
				httpClient: &http.Client{},
				logger:     &testLogger{},
			}

			err := client.DeleteCalendar(tt.calendarPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.expectedError != nil && err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCreateCalendarWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		authHeader: "Basic dGVzdDp0ZXN0",
		httpClient: &http.Client{},
		logger:     &testLogger{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calendar := &Calendar{
		DisplayName: "Test Calendar",
	}

	err := client.CreateCalendarWithContext(ctx, "/user/calendars", calendar)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got %v", err)
	}
}

func TestSanitizeCalendarName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test Calendar", "test-calendar"},
		{"My/Calendar", "my-calendar"},
		{"Calendar\\Name", "calendar-name"},
		{"Calendar:Name", "calendar-name"},
		{"Calendar*Name", "calendar-name"},
		{"Calendar?Name", "calendar-name"},
		{"Calendar\"Name", "calendar-name"},
		{"Calendar<Name>", "calendar-name-"},
		{"Calendar|Name", "calendar-name"},
		{"MixED CaSe NaMe", "mixed-case-name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeCalendarName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeCalendarName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildMakeCalendarXML(t *testing.T) {
	calendar := &Calendar{
		DisplayName:         "Test Calendar",
		Description:         "Test & Description",
		Color:               "#FF0000FF",
		CalendarTimeZone:    "Europe/London",
		SupportedComponents: []string{"VEVENT", "VTODO"},
	}

	xml := buildMakeCalendarXML(calendar)

	if !strings.Contains(xml, "<D:displayname>Test Calendar</D:displayname>") {
		t.Error("XML should contain display name")
	}

	if !strings.Contains(xml, "<C:calendar-description>Test &amp; Description</C:calendar-description>") {
		t.Error("XML should contain escaped description")
	}

	if !strings.Contains(xml, "<A:calendar-color>#FF0000FF</A:calendar-color>") {
		t.Error("XML should contain color")
	}

	if !strings.Contains(xml, "<C:calendar-timezone>Europe/London</C:calendar-timezone>") {
		t.Error("XML should contain timezone")
	}

	if !strings.Contains(xml, `<C:comp name="VEVENT"/>`) {
		t.Error("XML should contain VEVENT component")
	}

	if !strings.Contains(xml, `<C:comp name="VTODO"/>`) {
		t.Error("XML should contain VTODO component")
	}
}

func TestBuildUpdateCalendarXML(t *testing.T) {
	displayName := "Updated Name"
	description := "Updated & Description"
	color := "#00FF00FF"
	timezone := "America/New_York"

	updates := &CalendarPropertyUpdate{
		DisplayName:      &displayName,
		Description:      &description,
		Color:            &color,
		CalendarTimeZone: &timezone,
	}

	xml := buildUpdateCalendarXML(updates)

	if !strings.Contains(xml, "<D:displayname>Updated Name</D:displayname>") {
		t.Error("XML should contain display name")
	}

	if !strings.Contains(xml, "<C:calendar-description>Updated &amp; Description</C:calendar-description>") {
		t.Error("XML should contain escaped description")
	}

	if !strings.Contains(xml, "<A:calendar-color>#00FF00FF</A:calendar-color>") {
		t.Error("XML should contain color")
	}

	if !strings.Contains(xml, "<C:calendar-timezone>America/New_York</C:calendar-timezone>") {
		t.Error("XML should contain timezone")
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test & Test", "Test &amp; Test"},
		{"<tag>", "&lt;tag&gt;"},
		{"'quotes' and \"quotes\"", "&apos;quotes&apos; and &quot;quotes&quot;"},
		{"Normal text", "Normal text"},
		{"&<>\"'", "&amp;&lt;&gt;&quot;&apos;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeXML(tt.input)
			if result != tt.expected {
				t.Errorf("escapeXML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
