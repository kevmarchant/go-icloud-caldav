package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateEvent(t *testing.T) {
	tests := []struct {
		name          string
		event         *CalendarObject
		serverStatus  int
		serverHeaders map[string]string
		expectError   bool
		errorType     error
	}{
		{
			name: "successful creation",
			event: &CalendarObject{
				Summary:   "Test Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
				EndTime:   timePtrForCrud(time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)),
			},
			serverStatus: http.StatusCreated,
			serverHeaders: map[string]string{
				"ETag": `"test-etag-123"`,
			},
			expectError: false,
		},
		{
			name: "event already exists",
			event: &CalendarObject{
				UID:       "existing-uid",
				Summary:   "Test Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			serverStatus: http.StatusPreconditionFailed,
			expectError:  true,
			errorType:    &EventExistsError{},
		},
		{
			name: "missing summary",
			event: &CalendarObject{
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			expectError: true,
		},
		{
			name: "missing start time",
			event: &CalendarObject{
				Summary: "Test Event",
			},
			expectError: true,
		},
		{
			name:        "nil event",
			event:       nil,
			expectError: true,
		},
		{
			name: "server error",
			event: &CalendarObject{
				Summary:   "Test Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			var capturedBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				body := make([]byte, 4096)
				n, _ := r.Body.Read(body)
				capturedBody = string(body[:n])

				for key, value := range tt.serverHeaders {
					w.Header().Set(key, value)
				}
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.SetBaseURL(server.URL)

			err := client.CreateEvent("/calendars/test", tt.event)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != nil {
					switch tt.errorType.(type) {
					case *EventExistsError:
						if _, ok := err.(*EventExistsError); !ok {
							t.Errorf("expected EventExistsError, got %T", err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if capturedRequest != nil {
					if capturedRequest.Method != "PUT" {
						t.Errorf("expected PUT method, got %s", capturedRequest.Method)
					}

					if capturedRequest.Header.Get("If-None-Match") != "*" {
						t.Errorf("expected If-None-Match: *, got %s", capturedRequest.Header.Get("If-None-Match"))
					}

					if capturedRequest.Header.Get("Content-Type") != "text/calendar; charset=utf-8" {
						t.Errorf("expected Content-Type: text/calendar; charset=utf-8, got %s", capturedRequest.Header.Get("Content-Type"))
					}

					if !strings.Contains(capturedBody, "BEGIN:VCALENDAR") {
						t.Error("request body missing BEGIN:VCALENDAR")
					}
					if !strings.Contains(capturedBody, "BEGIN:VEVENT") {
						t.Error("request body missing BEGIN:VEVENT")
					}
					if tt.event != nil && tt.event.Summary != "" && !strings.Contains(capturedBody, fmt.Sprintf("SUMMARY:%s", tt.event.Summary)) {
						t.Errorf("request body missing SUMMARY:%s", tt.event.Summary)
					}
				}

				if tt.event != nil && tt.serverHeaders["ETag"] != "" {
					if tt.event.ETag != tt.serverHeaders["ETag"] {
						t.Errorf("expected ETag %s, got %s", tt.serverHeaders["ETag"], tt.event.ETag)
					}
				}
			}
		})
	}
}

func TestUpdateEvent(t *testing.T) {
	tests := []struct {
		name          string
		event         *CalendarObject
		etag          string
		serverStatus  int
		serverHeaders map[string]string
		expectError   bool
		errorType     error
	}{
		{
			name: "successful update",
			event: &CalendarObject{
				UID:       "test-uid",
				Summary:   "Updated Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			etag:         `"old-etag"`,
			serverStatus: http.StatusNoContent,
			serverHeaders: map[string]string{
				"ETag": `"new-etag"`,
			},
			expectError: false,
		},
		{
			name: "etag mismatch",
			event: &CalendarObject{
				UID:       "test-uid",
				Summary:   "Updated Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			etag:         `"old-etag"`,
			serverStatus: http.StatusPreconditionFailed,
			expectError:  true,
			errorType:    &ETagMismatchError{},
		},
		{
			name: "event not found",
			event: &CalendarObject{
				UID:       "nonexistent-uid",
				Summary:   "Updated Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			serverStatus: http.StatusNotFound,
			expectError:  true,
			errorType:    &EventNotFoundError{},
		},
		{
			name: "missing UID",
			event: &CalendarObject{
				Summary:   "Updated Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			expectError: true,
		},
		{
			name: "update without etag",
			event: &CalendarObject{
				UID:       "test-uid",
				Summary:   "Force Updated Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			etag:         "",
			serverStatus: http.StatusNoContent,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request
			var capturedBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				body := make([]byte, 4096)
				n, _ := r.Body.Read(body)
				capturedBody = string(body[:n])

				for key, value := range tt.serverHeaders {
					w.Header().Set(key, value)
				}
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.SetBaseURL(server.URL)

			err := client.UpdateEvent("/calendars/test", tt.event, tt.etag)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != nil {
					switch tt.errorType.(type) {
					case *ETagMismatchError:
						if _, ok := err.(*ETagMismatchError); !ok {
							t.Errorf("expected ETagMismatchError, got %T", err)
						}
					case *EventNotFoundError:
						if _, ok := err.(*EventNotFoundError); !ok {
							t.Errorf("expected EventNotFoundError, got %T", err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if capturedRequest != nil {
					if capturedRequest.Method != "PUT" {
						t.Errorf("expected PUT method, got %s", capturedRequest.Method)
					}

					if tt.etag != "" && capturedRequest.Header.Get("If-Match") != tt.etag {
						t.Errorf("expected If-Match: %s, got %s", tt.etag, capturedRequest.Header.Get("If-Match"))
					}

					if !strings.Contains(capturedBody, "LAST-MODIFIED:") {
						t.Error("request body missing LAST-MODIFIED property")
					}
				}

				if tt.event != nil && tt.serverHeaders["ETag"] != "" {
					if tt.event.ETag != tt.serverHeaders["ETag"] {
						t.Errorf("expected ETag %s, got %s", tt.serverHeaders["ETag"], tt.event.ETag)
					}
				}
			}
		})
	}
}

func TestDeleteEvent(t *testing.T) {
	tests := []struct {
		name         string
		eventPath    string
		serverStatus int
		expectError  bool
	}{
		{
			name:         "successful deletion",
			eventPath:    "/calendars/test/event-uid.ics",
			serverStatus: http.StatusNoContent,
			expectError:  false,
		},
		{
			name:         "event not found (idempotent)",
			eventPath:    "/calendars/test/nonexistent.ics",
			serverStatus: http.StatusNotFound,
			expectError:  false,
		},
		{
			name:         "server error",
			eventPath:    "/calendars/test/event-uid.ics",
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
		{
			name:         "path without .ics extension",
			eventPath:    "/calendars/test/event-uid",
			serverStatus: http.StatusNoContent,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.SetBaseURL(server.URL)

			err := client.DeleteEvent(tt.eventPath)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if capturedRequest != nil {
					if capturedRequest.Method != "DELETE" {
						t.Errorf("expected DELETE method, got %s", capturedRequest.Method)
					}

					if !strings.HasSuffix(capturedRequest.URL.Path, ".ics") {
						t.Errorf("expected path to end with .ics, got %s", capturedRequest.URL.Path)
					}
				}
			}
		})
	}
}

func TestDeleteEventWithETag(t *testing.T) {
	tests := []struct {
		name         string
		eventPath    string
		etag         string
		serverStatus int
		expectError  bool
		errorType    error
	}{
		{
			name:         "successful deletion with etag",
			eventPath:    "/calendars/test/event-uid.ics",
			etag:         `"test-etag"`,
			serverStatus: http.StatusNoContent,
			expectError:  false,
		},
		{
			name:         "etag mismatch",
			eventPath:    "/calendars/test/event-uid.ics",
			etag:         `"wrong-etag"`,
			serverStatus: http.StatusPreconditionFailed,
			expectError:  true,
			errorType:    &ETagMismatchError{},
		},
		{
			name:         "force deletion without etag",
			eventPath:    "/calendars/test/event-uid.ics",
			etag:         "",
			serverStatus: http.StatusNoContent,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequest *http.Request

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedRequest = r
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.SetBaseURL(server.URL)

			err := client.DeleteEventWithETag(context.Background(), tt.eventPath, tt.etag)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if tt.errorType != nil {
					switch tt.errorType.(type) {
					case *ETagMismatchError:
						if _, ok := err.(*ETagMismatchError); !ok {
							t.Errorf("expected ETagMismatchError, got %T", err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if capturedRequest != nil && tt.etag != "" {
					if capturedRequest.Header.Get("If-Match") != tt.etag {
						t.Errorf("expected If-Match: %s, got %s", tt.etag, capturedRequest.Header.Get("If-Match"))
					}
				}
			}
		})
	}
}

func TestGenerateICalendar(t *testing.T) {
	tests := []struct {
		name        string
		event       *CalendarObject
		expectError bool
		contains    []string
		notContains []string
	}{
		{
			name: "complete event",
			event: &CalendarObject{
				UID:         "test-uid",
				Summary:     "Test Event",
				Description: "This is a test event",
				Location:    "Test Location",
				StartTime:   timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
				EndTime:     timePtrForCrud(time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)),
				Status:      "CONFIRMED",
				Organizer:   "mailto:organizer@example.com",
				Attendees:   []string{"mailto:attendee1@example.com", "mailto:attendee2@example.com"},
			},
			expectError: false,
			contains: []string{
				"BEGIN:VCALENDAR",
				"VERSION:2.0",
				"PRODID:-//go-icloud-caldav//EN",
				"BEGIN:VEVENT",
				"UID:test-uid",
				"SUMMARY:Test Event",
				"DESCRIPTION:This is a test event",
				"LOCATION:Test Location",
				"DTSTART:20240115T100000Z",
				"DTEND:20240115T110000Z",
				"STATUS:CONFIRMED",
				"ORGANIZER:mailto:organizer@example.com",
				"ATTENDEE:mailto:attendee1@example.com",
				"ATTENDEE:mailto:attendee2@example.com",
				"END:VEVENT",
				"END:VCALENDAR",
			},
		},
		{
			name: "minimal event",
			event: &CalendarObject{
				Summary:   "Minimal Event",
				StartTime: timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			expectError: false,
			contains: []string{
				"BEGIN:VCALENDAR",
				"BEGIN:VEVENT",
				"SUMMARY:Minimal Event",
				"DTSTART:20240115T100000Z",
				"DTEND:20240115T110000Z",
				"STATUS:CONFIRMED",
				"END:VEVENT",
				"END:VCALENDAR",
			},
		},
		{
			name: "event with special characters",
			event: &CalendarObject{
				Summary:     "Event; with, special\\chars\nand newline",
				Description: "Description with\nmultiple\nlines",
				StartTime:   timePtrForCrud(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			},
			expectError: false,
			contains: []string{
				"SUMMARY:Event\\; with\\, special\\\\chars\\nand newline",
				"DESCRIPTION:Description with\\nmultiple\\nlines",
			},
		},
		{
			name:        "nil event",
			event:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateICalendar(tt.event)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				for _, expected := range tt.contains {
					if !strings.Contains(result, expected) {
						t.Errorf("expected result to contain %q", expected)
					}
				}

				for _, unexpected := range tt.notContains {
					if strings.Contains(result, unexpected) {
						t.Errorf("expected result not to contain %q", unexpected)
					}
				}

				lines := strings.Split(result, "\r\n")
				for _, line := range lines {
					if len(line) > 75 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
						t.Logf("Warning: line exceeds 75 characters: %s", line)
					}
				}
			}
		})
	}
}

func TestBuildEventURL(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		calendarPath string
		uid          string
		expected     string
	}{
		{
			name:         "standard path",
			baseURL:      "https://caldav.example.com",
			calendarPath: "/calendars/user/default",
			uid:          "event-123",
			expected:     "https://caldav.example.com/calendars/user/default/event-123.ics",
		},
		{
			name:         "path without leading slash",
			baseURL:      "https://caldav.example.com",
			calendarPath: "calendars/user/default",
			uid:          "event-123",
			expected:     "https://caldav.example.com/calendars/user/default/event-123.ics",
		},
		{
			name:         "path without trailing slash",
			baseURL:      "https://caldav.example.com",
			calendarPath: "/calendars/user/default",
			uid:          "event-123",
			expected:     "https://caldav.example.com/calendars/user/default/event-123.ics",
		},
		{
			name:         "path with both slashes",
			baseURL:      "https://caldav.example.com",
			calendarPath: "/calendars/user/default/",
			uid:          "event-123",
			expected:     "https://caldav.example.com/calendars/user/default/event-123.ics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildEventURL(tt.baseURL, tt.calendarPath, tt.uid)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGenerateUID(t *testing.T) {
	uid1 := generateUID()
	uid2 := generateUID()

	if uid1 == "" {
		t.Error("generateUID returned empty string")
	}

	if uid2 == "" {
		t.Error("generateUID returned empty string")
	}

	if uid1 == uid2 {
		t.Error("generateUID returned duplicate UIDs")
	}

	if !strings.HasSuffix(uid1, "@go-icloud-caldav") {
		t.Errorf("UID should end with @go-icloud-caldav, got %s", uid1)
	}
}

func timePtrForCrud(t time.Time) *time.Time {
	return &t
}
