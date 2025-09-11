package caldav

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type testLogger struct {
	debugMessages []string
	infoMessages  []string
	errorMessages []string
}

func (t *testLogger) Debug(msg string, keysAndValues ...interface{}) {
	t.debugMessages = append(t.debugMessages, msg)
}

func (t *testLogger) Info(msg string, keysAndValues ...interface{}) {
	t.infoMessages = append(t.infoMessages, msg)
}

func (t *testLogger) Warn(msg string, keysAndValues ...interface{}) {}

func (t *testLogger) Error(msg string, keysAndValues ...interface{}) {
	t.errorMessages = append(t.errorMessages, msg)
}

func TestClientWithLogger(t *testing.T) {
	testLogger := &testLogger{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/principal/</D:href>
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
	}))
	defer server.Close()

	client := NewClientWithOptions("user", "pass", WithLogger(testLogger))
	client.baseURL = server.URL

	_, err := client.FindCurrentUserPrincipal(context.Background())
	if err != nil {
		t.Fatalf("FindCurrentUserPrincipal() error = %v", err)
	}

}

func TestClientWithDebugLogging(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/principal/</D:href>
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
	}))
	defer server.Close()

	var buf bytes.Buffer
	client := NewClientWithOptions("user", "pass", WithDebugLogging(&buf))
	client.baseURL = server.URL

	if client.logger == nil {
		t.Errorf("Expected logger to be set")
	}

	_, err := client.FindCurrentUserPrincipal(context.Background())
	if err != nil {
		t.Fatalf("FindCurrentUserPrincipal() error = %v", err)
	}
}

func TestClientWithDebugLoggingStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/principal/</D:href>
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
	}))
	defer server.Close()

	client := NewClientWithOptions("user", "pass", WithDebugLogging(os.Stdout))
	client.baseURL = server.URL

	if client.logger == nil {
		t.Errorf("Expected logger to be set")
	}
}

func TestClientLoggingOnError(t *testing.T) {
	testLogger := &testLogger{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewClientWithOptions("user", "pass", WithLogger(testLogger))
	client.baseURL = server.URL

	_, err := client.FindCurrentUserPrincipal(context.Background())
	if err == nil {
		t.Fatalf("Expected error but got none")
	}
}
