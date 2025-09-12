package caldav

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPropfindErrors(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "network error",
			statusCode:   0,
			responseBody: "",
			expectError:  true,
		},
		{
			name:         "404 not found",
			statusCode:   404,
			responseBody: "Not Found",
			expectError:  true,
		},
		{
			name:         "500 server error",
			statusCode:   500,
			responseBody: "Internal Server Error",
			expectError:  true,
		},
		{
			name:         "401 unauthorized",
			statusCode:   401,
			responseBody: "Unauthorized",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("user", "pass")
			xmlBody := []byte(`<?xml version="1.0"?><propfind/>`)

			if tt.statusCode == 0 {
				// Simulate network error with invalid URL
				client.baseURL = "http://[::1]:0"
				resp, err := client.propfind(context.Background(), "/test", "0", xmlBody)
				if err == nil {
					t.Errorf("Expected error for network failure but got none")
				}
				if resp != nil {
					defer func() { _ = resp.Body.Close() }()
				}
				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client.baseURL = server.URL
			resp, err := client.propfind(context.Background(), "/test", "0", xmlBody)

			if err != nil {
				t.Errorf("Expected no error for status %d but got: %v", tt.statusCode, err)
			}
			if resp == nil {
				t.Errorf("Expected response for status %d but got nil", tt.statusCode)
			} else if resp.StatusCode != tt.statusCode {
				t.Errorf("Expected status code %d but got %d", tt.statusCode, resp.StatusCode)
			}

			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
		})
	}
}

func TestReportErrors(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectError  bool
	}{
		{
			name:         "network error",
			statusCode:   0,
			responseBody: "",
			expectError:  true,
		},
		{
			name:         "404 not found",
			statusCode:   404,
			responseBody: "Not Found",
			expectError:  true,
		},
		{
			name:         "403 forbidden",
			statusCode:   403,
			responseBody: "Forbidden",
			expectError:  true,
		},
		{
			name:         "502 bad gateway",
			statusCode:   502,
			responseBody: "Bad Gateway",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("user", "pass")
			xmlBody := []byte(`<?xml version="1.0"?><calendar-query/>`)

			if tt.statusCode == 0 {
				// Simulate network error with invalid URL
				client.baseURL = "http://[::1]:0"
				resp, err := client.report(context.Background(), "/test", xmlBody)
				if err == nil {
					t.Errorf("Expected error for network failure but got none")
				}
				if resp != nil {
					defer func() { _ = resp.Body.Close() }()
				}
				return
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client.baseURL = server.URL
			resp, err := client.report(context.Background(), "/test", xmlBody)

			if err != nil {
				t.Errorf("Expected no error for status %d but got: %v", tt.statusCode, err)
			}
			if resp == nil {
				t.Errorf("Expected response for status %d but got nil", tt.statusCode)
			} else if resp.StatusCode != tt.statusCode {
				t.Errorf("Expected status code %d but got %d", tt.statusCode, resp.StatusCode)
			}

			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
		})
	}
}

func TestPropfindContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
		w.WriteHeader(207)
	}))
	defer server.Close()
	defer close(blocked)

	client := NewClient("user", "pass")
	client.baseURL = server.URL
	client.SetTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	xmlBody := []byte(`<?xml version="1.0"?><propfind/>`)
	resp, err := client.propfind(ctx, "/test", "0", xmlBody)

	if err == nil {
		t.Error("Expected context cancellation error")
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

func TestReportContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
		w.WriteHeader(207)
	}))
	defer server.Close()
	defer close(blocked)

	client := NewClient("user", "pass")
	client.baseURL = server.URL
	client.SetTimeout(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	xmlBody := []byte(`<?xml version="1.0"?><calendar-query/>`)
	resp, err := client.report(ctx, "/test", xmlBody)

	if err == nil {
		t.Error("Expected context cancellation error")
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

func TestPropfindWithLoggerDebug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><multistatus/>`))
	}))
	defer server.Close()

	logOutput := &strings.Builder{}
	logger := &testLoggerAlt{output: logOutput}

	client := NewClientWithOptions(
		"user",
		"pass",
		WithLogger(logger),
	)
	client.baseURL = server.URL
	client.debugHTTP = true

	xmlBody := []byte(`<?xml version="1.0"?><propfind/>`)
	resp, err := client.propfind(context.Background(), "/test", "0", xmlBody)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logStr := logOutput.String()
	if !strings.Contains(logStr, "PROPFIND") {
		t.Errorf("Expected PROPFIND in log output, got: %s", logStr)
	}
	if !strings.Contains(logStr, "HTTP Request:") {
		t.Errorf("Expected HTTP Request in debug log, got: %s", logStr)
	}
	if !strings.Contains(logStr, "HTTP Response:") {
		t.Errorf("Expected HTTP Response in debug log, got: %s", logStr)
	}
}

func TestReportWithLoggerDebug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><multistatus/>`))
	}))
	defer server.Close()

	logOutput := &strings.Builder{}
	logger := &testLoggerAlt{output: logOutput}

	client := NewClientWithOptions(
		"user",
		"pass",
		WithLogger(logger),
	)
	client.baseURL = server.URL
	client.debugHTTP = true

	xmlBody := []byte(`<?xml version="1.0"?><calendar-query/>`)
	resp, err := client.report(context.Background(), "/test", xmlBody)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logStr := logOutput.String()
	if !strings.Contains(logStr, "REPORT") {
		t.Errorf("Expected REPORT in log output, got: %s", logStr)
	}
	if !strings.Contains(logStr, "HTTP Request:") {
		t.Errorf("Expected HTTP Request in debug log, got: %s", logStr)
	}
	if !strings.Contains(logStr, "HTTP Response:") {
		t.Errorf("Expected HTTP Response in debug log, got: %s", logStr)
	}
}

type testLoggerAlt struct {
	output *strings.Builder
}

func (l *testLoggerAlt) Info(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.output.WriteString("INFO: " + formatted + "\n")
}

func (l *testLoggerAlt) Error(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.output.WriteString("ERROR: " + formatted + "\n")
}

func (l *testLoggerAlt) Warn(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.output.WriteString("WARN: " + formatted + "\n")
}

func (l *testLoggerAlt) Debug(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.output.WriteString("DEBUG: " + formatted + "\n")
}
