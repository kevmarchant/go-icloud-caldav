package caldav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupTestClient(t *testing.T, handler http.HandlerFunc) (*CalDAVClient, *httptest.Server) {
	server := httptest.NewServer(handler)
	client := NewClient("test@example.com", "password")
	client.SetBaseURL(server.URL)
	return client, server
}

func TestCreateTodo(t *testing.T) {
	tests := []struct {
		name           string
		todo           *ParsedTodo
		responseStatus int
		responseBody   string
		wantErr        bool
		expectedError  error
	}{
		{
			name: "successful creation",
			todo: &ParsedTodo{
				Summary:         "Test Task",
				Description:     "Test Description",
				Status:          "NEEDS-ACTION",
				Priority:        5,
				PercentComplete: 0,
			},
			responseStatus: http.StatusCreated,
			wantErr:        false,
		},
		{
			name: "todo already exists",
			todo: &ParsedTodo{
				UID:     "existing-todo",
				Summary: "Test Task",
			},
			responseStatus: http.StatusPreconditionFailed,
			wantErr:        true,
			expectedError:  &EventExistsError{UID: "existing-todo"},
		},
		{
			name: "unauthorized",
			todo: &ParsedTodo{
				Summary: "Test Task",
			},
			responseStatus: http.StatusUnauthorized,
			wantErr:        true,
			expectedError:  ErrUnauthorized,
		},
		{
			name: "missing summary",
			todo: &ParsedTodo{
				Description: "Description without summary",
			},
			wantErr:       true,
			expectedError: fmt.Errorf("summary is required"),
		},
		{
			name: "invalid priority",
			todo: &ParsedTodo{
				Summary:  "Test Task",
				Priority: 10,
			},
			wantErr:       true,
			expectedError: fmt.Errorf("priority must be between 0 and 9"),
		},
		{
			name: "invalid percent complete",
			todo: &ParsedTodo{
				Summary:         "Test Task",
				PercentComplete: 150,
			},
			wantErr:       true,
			expectedError: fmt.Errorf("percent complete must be between 0 and 100"),
		},
		{
			name: "invalid status",
			todo: &ParsedTodo{
				Summary: "Test Task",
				Status:  "INVALID",
			},
			wantErr:       true,
			expectedError: fmt.Errorf("invalid status: INVALID"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}

				if r.Header.Get("Content-Type") != "text/calendar; charset=utf-8" {
					t.Errorf("expected Content-Type header, got %s", r.Header.Get("Content-Type"))
				}

				if r.Header.Get("If-None-Match") != "*" {
					t.Errorf("expected If-None-Match header to be *, got %s", r.Header.Get("If-None-Match"))
				}

				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "BEGIN:VTODO") {
					t.Errorf("expected VTODO in body, got %s", string(body))
				}

				if tt.todo != nil && tt.todo.Summary != "" && !strings.Contains(string(body), "SUMMARY:"+tt.todo.Summary) {
					t.Errorf("expected SUMMARY in body, got %s", string(body))
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			})
			defer server.Close()

			err := client.CreateTodo(context.Background(), "/calendars/test/", tt.todo)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectedError != nil {
					switch expected := tt.expectedError.(type) {
					case *EventExistsError:
						var exists *EventExistsError
						if !errors.As(err, &exists) {
							t.Errorf("expected EventExistsError, got %v", err)
						}
					case error:
						if !strings.Contains(err.Error(), expected.Error()) {
							t.Errorf("expected error %v, got %v", expected, err)
						}
					}
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateTodo(t *testing.T) {
	tests := []struct {
		name           string
		todo           *ParsedTodo
		etag           string
		responseStatus int
		wantErr        bool
		expectedError  error
	}{
		{
			name: "successful update",
			todo: &ParsedTodo{
				UID:             "test-todo",
				Summary:         "Updated Task",
				Description:     "Updated Description",
				Status:          "IN-PROCESS",
				PercentComplete: 50,
			},
			etag:           "\"123456\"",
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name: "concurrent modification",
			todo: &ParsedTodo{
				UID:     "test-todo",
				Summary: "Updated Task",
			},
			etag:           "\"old-etag\"",
			responseStatus: http.StatusPreconditionFailed,
			wantErr:        true,
			expectedError:  &ETagMismatchError{Expected: "\"old-etag\""},
		},
		{
			name: "todo not found",
			todo: &ParsedTodo{
				UID:     "non-existent",
				Summary: "Task",
			},
			responseStatus: http.StatusNotFound,
			wantErr:        true,
			expectedError:  &EventNotFoundError{UID: "non-existent"},
		},
		{
			name: "missing UID",
			todo: &ParsedTodo{
				Summary: "Task without UID",
			},
			wantErr:       true,
			expectedError: fmt.Errorf("UID is required for update"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}

				if tt.etag != "" && r.Header.Get("If-Match") != tt.etag {
					t.Errorf("expected If-Match header %s, got %s", tt.etag, r.Header.Get("If-Match"))
				}

				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "BEGIN:VTODO") {
					t.Errorf("expected VTODO in body")
				}

				w.WriteHeader(tt.responseStatus)
			})
			defer server.Close()

			err := client.UpdateTodo(context.Background(), "/calendars/test/", tt.todo, tt.etag)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectedError != nil {
					switch expected := tt.expectedError.(type) {
					case *EventExistsError:
						var exists *EventExistsError
						if !errors.As(err, &exists) {
							t.Errorf("expected EventExistsError, got %v", err)
						}
					case error:
						if !strings.Contains(err.Error(), expected.Error()) {
							t.Errorf("expected error %v, got %v", expected, err)
						}
					}
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteTodo(t *testing.T) {
	tests := []struct {
		name           string
		etag           string
		responseStatus int
		wantErr        bool
		expectedError  error
	}{
		{
			name:           "successful deletion",
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name:           "todo not found (idempotent)",
			responseStatus: http.StatusNotFound,
			wantErr:        false,
		},
		{
			name:           "concurrent modification",
			etag:           "\"old-etag\"",
			responseStatus: http.StatusPreconditionFailed,
			wantErr:        true,
			expectedError:  &ETagMismatchError{Expected: "\"old-etag\""},
		},
		{
			name:           "unauthorized",
			responseStatus: http.StatusUnauthorized,
			wantErr:        true,
			expectedError:  ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("expected DELETE request, got %s", r.Method)
				}

				if tt.etag != "" && r.Header.Get("If-Match") != tt.etag {
					t.Errorf("expected If-Match header %s, got %s", tt.etag, r.Header.Get("If-Match"))
				}

				w.WriteHeader(tt.responseStatus)
			})
			defer server.Close()

			err := client.DeleteTodo(context.Background(), "/calendars/test/todo.ics", tt.etag)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectedError != nil {
					switch expected := tt.expectedError.(type) {
					case *ETagMismatchError:
						var etag *ETagMismatchError
						if !errors.As(err, &etag) {
							t.Errorf("expected ETagMismatchError, got %v", err)
						}
					default:
						if err != expected {
							t.Errorf("expected error %v, got %v", expected, err)
						}
					}
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCompleteTodo(t *testing.T) {
	existingTodo := &ParsedTodo{
		UID:             "test-todo",
		Summary:         "Test Task",
		Status:          "NEEDS-ACTION",
		PercentComplete: 0,
	}

	tests := []struct {
		name            string
		uid             string
		percentComplete int
		expectedStatus  string
		expectedPercent int
		wantErr         bool
	}{
		{
			name:            "mark as completed",
			uid:             "test-todo",
			percentComplete: 100,
			expectedStatus:  "COMPLETED",
			expectedPercent: 100,
			wantErr:         false,
		},
		{
			name:            "mark as in-process",
			uid:             "test-todo",
			percentComplete: 50,
			expectedStatus:  "IN-PROCESS",
			expectedPercent: 50,
			wantErr:         false,
		},
		{
			name:            "reset to needs-action",
			uid:             "test-todo",
			percentComplete: 0,
			expectedStatus:  "NEEDS-ACTION",
			expectedPercent: 0,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if callCount == 1 {
					if r.Method != http.MethodGet {
						t.Errorf("expected GET request for first call, got %s", r.Method)
					}
					icalData := generateTodoICalendar(existingTodo)
					w.Header().Set("ETag", "\"test-etag\"")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(icalData))
				} else {
					if r.Method != http.MethodPut {
						t.Errorf("expected PUT request for second call, got %s", r.Method)
					}
					body, _ := io.ReadAll(r.Body)
					bodyStr := string(body)

					if tt.expectedStatus == "COMPLETED" && !strings.Contains(bodyStr, "STATUS:COMPLETED") {
						t.Errorf("expected STATUS:COMPLETED in body")
					}
					if tt.expectedPercent > 0 && !strings.Contains(bodyStr, fmt.Sprintf("PERCENT-COMPLETE:%d", tt.expectedPercent)) {
						t.Errorf("expected PERCENT-COMPLETE:%d in body", tt.expectedPercent)
					}

					w.WriteHeader(http.StatusNoContent)
				}
			})
			defer server.Close()

			err := client.CompleteTodo(context.Background(), "/calendars/test/", tt.uid, tt.percentComplete)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetTodo(t *testing.T) {
	testTodo := &ParsedTodo{
		UID:         "test-todo",
		Summary:     "Test Task",
		Description: "Test Description",
		Status:      "NEEDS-ACTION",
	}

	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		responseETag   string
		wantErr        bool
		expectedError  error
	}{
		{
			name:           "successful retrieval",
			responseStatus: http.StatusOK,
			responseBody:   generateTodoICalendar(testTodo),
			responseETag:   "\"test-etag\"",
			wantErr:        false,
		},
		{
			name:           "todo not found",
			responseStatus: http.StatusNotFound,
			wantErr:        true,
			expectedError:  &EventNotFoundError{UID: "non-existent"},
		},
		{
			name:           "invalid iCalendar",
			responseStatus: http.StatusOK,
			responseBody:   "invalid ical data",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET request, got %s", r.Method)
				}

				if tt.responseETag != "" {
					w.Header().Set("ETag", tt.responseETag)
				}
				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			})
			defer server.Close()

			todo, etag, err := client.GetTodo(context.Background(), "/calendars/test/todo.ics")

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectedError != nil {
					switch expected := tt.expectedError.(type) {
					case *EventNotFoundError:
						var notFound *EventNotFoundError
						if !errors.As(err, &notFound) {
							t.Errorf("expected EventNotFoundError, got %v", err)
						}
					default:
						if !strings.Contains(err.Error(), expected.Error()) {
							t.Errorf("expected error %v, got %v", expected, err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if todo == nil {
					t.Errorf("expected todo, got nil")
				} else if todo.UID != testTodo.UID {
					t.Errorf("expected UID %s, got %s", testTodo.UID, todo.UID)
				}
				if etag != tt.responseETag {
					t.Errorf("expected etag %s, got %s", tt.responseETag, etag)
				}
			}
		})
	}
}

func TestGetTodos(t *testing.T) {
	todo1 := &ParsedTodo{
		UID:     "todo-1",
		Summary: "Task 1",
		Status:  "NEEDS-ACTION",
	}
	todo2 := &ParsedTodo{
		UID:     "todo-2",
		Summary: "Task 2",
		Status:  "COMPLETED",
	}

	multiStatusResponse := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/test/todo1.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag1"</D:getetag>
        <C:calendar-data>%s</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/calendars/test/todo2.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"etag2"</D:getetag>
        <C:calendar-data>%s</C:calendar-data>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, generateTodoICalendar(todo1), generateTodoICalendar(todo2))

	client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("expected REPORT request, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "VTODO") {
			t.Errorf("expected VTODO filter in request body")
		}

		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(multiStatusResponse))
	})
	defer server.Close()

	todos, err := client.GetTodos(context.Background(), "/calendars/test/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(todos) != 2 {
		t.Errorf("expected 2 todos, got %d", len(todos))
	}

	if todos[0].UID != todo1.UID {
		t.Errorf("expected first todo UID %s, got %s", todo1.UID, todos[0].UID)
	}
	if todos[1].UID != todo2.UID {
		t.Errorf("expected second todo UID %s, got %s", todo2.UID, todos[1].UID)
	}
}

func TestValidateTodo(t *testing.T) {
	tests := []struct {
		name    string
		todo    *ParsedTodo
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid todo",
			todo: &ParsedTodo{
				Summary:         "Valid Task",
				Priority:        5,
				PercentComplete: 50,
				Status:          "IN-PROCESS",
			},
			wantErr: false,
		},
		{
			name:    "nil todo",
			todo:    nil,
			wantErr: true,
			errMsg:  "todo cannot be nil",
		},
		{
			name: "missing summary",
			todo: &ParsedTodo{
				Description: "No summary",
			},
			wantErr: true,
			errMsg:  "summary is required",
		},
		{
			name: "invalid priority too high",
			todo: &ParsedTodo{
				Summary:  "Task",
				Priority: 10,
			},
			wantErr: true,
			errMsg:  "priority must be between 0 and 9",
		},
		{
			name: "invalid priority negative",
			todo: &ParsedTodo{
				Summary:  "Task",
				Priority: -1,
			},
			wantErr: true,
			errMsg:  "priority must be between 0 and 9",
		},
		{
			name: "invalid percent complete too high",
			todo: &ParsedTodo{
				Summary:         "Task",
				PercentComplete: 101,
			},
			wantErr: true,
			errMsg:  "percent complete must be between 0 and 100",
		},
		{
			name: "invalid percent complete negative",
			todo: &ParsedTodo{
				Summary:         "Task",
				PercentComplete: -1,
			},
			wantErr: true,
			errMsg:  "percent complete must be between 0 and 100",
		},
		{
			name: "invalid status",
			todo: &ParsedTodo{
				Summary: "Task",
				Status:  "INVALID-STATUS",
			},
			wantErr: true,
			errMsg:  "invalid status: INVALID-STATUS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTodo(tt.todo)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateTodoICalendar(t *testing.T) {
	now := time.Now().UTC()
	todo := &ParsedTodo{
		UID:             "test-uid",
		DTStamp:         &now,
		Created:         &now,
		LastModified:    &now,
		Summary:         "Test Task",
		Description:     "Test Description",
		Status:          "IN-PROCESS",
		Priority:        3,
		PercentComplete: 50,
		DTStart:         &now,
		Due:             &now,
		Completed:       nil,
		Sequence:        1,
		Class:           "PRIVATE",
		Categories:      []string{"Work", "Important"},
		Contacts:        []string{"John Doe", "Jane Smith"},
		Comments:        []string{"First comment", "Second comment"},
		RelatedTo: []RelatedEvent{
			{UID: "parent-uid", RelationType: "PARENT"},
			{UID: "sibling-uid"},
		},
		Attachments: []Attachment{
			{URI: "https://example.com/file.pdf"},
			{Value: "base64data", FormatType: "image/png"},
		},
		RequestStatus: []RequestStatus{
			{Code: "2.0", Description: "Success"},
		},
	}

	ical := generateTodoICalendar(todo)

	requiredFields := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//go-icloud-caldav//EN",
		"BEGIN:VTODO",
		"UID:test-uid",
		"SUMMARY:Test Task",
		"DESCRIPTION:Test Description",
		"STATUS:IN-PROCESS",
		"PRIORITY:3",
		"PERCENT-COMPLETE:50",
		"SEQUENCE:1",
		"CLASS:PRIVATE",
		"CATEGORIES:Work",
		"CATEGORIES:Important",
		"CONTACT:John Doe",
		"CONTACT:Jane Smith",
		"COMMENT:First comment",
		"COMMENT:Second comment",
		"RELATED-TO;RELTYPE=PARENT:parent-uid",
		"RELATED-TO:sibling-uid",
		"ATTACH:https://example.com/file.pdf",
		"ATTACH;ENCODING=BASE64;VALUE=BINARY:base64data",
		"REQUEST-STATUS:2.0;Success",
		"END:VTODO",
		"END:VCALENDAR",
	}

	for _, field := range requiredFields {
		if !strings.Contains(ical, field) {
			t.Errorf("expected field '%s' in iCalendar, got:\n%s", field, ical)
		}
	}
}

func TestBuildTodoURL(t *testing.T) {
	tests := []struct {
		name         string
		calendarPath string
		uid          string
		expected     string
	}{
		{
			name:         "path with trailing slash",
			calendarPath: "/calendars/test/",
			uid:          "todo-123",
			expected:     "https://example.com/calendars/test/todo-123.ics",
		},
		{
			name:         "path without trailing slash",
			calendarPath: "/calendars/test",
			uid:          "todo-456",
			expected:     "https://example.com/calendars/test/todo-456.ics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTodoURL("https://example.com", tt.calendarPath, tt.uid)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDeleteTodoByUID(t *testing.T) {
	client, server := setupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE request, got %s", r.Method)
		}
		expectedPath := "/calendars/test/test-todo.ics"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	err := client.DeleteTodoByUID(context.Background(), "/calendars/test/", "test-todo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
