package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBatchProcessor_ExecuteBatch(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>` + r.URL.Path + `</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Test Calendar</D:displayname>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	processor := NewBatchProcessor(client, 2, 30*time.Second, 3)

	requests := []BatchRequest{
		{Path: "/calendar1/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
		{Path: "/calendar2/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
		{Path: "/calendar3/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
		{Path: "/calendar4/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
	}

	responses, err := processor.ExecuteBatch(context.Background(), requests)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if len(responses) != 4 {
		t.Errorf("expected 4 responses, got %d", len(responses))
	}

	for i, resp := range responses {
		if resp.Error != nil {
			t.Errorf("response %d has error: %v", i, resp.Error)
		}
		if resp.Response == nil {
			t.Errorf("response %d has nil response", i)
		}
	}

	if requestCount != 4 {
		t.Errorf("expected 4 HTTP requests, got %d", requestCount)
	}
}

func TestBatchProcessor_OptimizeBatch(t *testing.T) {
	client := NewClient("test", "test")
	processor := NewBatchProcessor(client, 10, 30*time.Second, 3)

	requests := []BatchRequest{
		{Path: "/cal1/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
		{Path: "/cal2/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
		{Path: "/cal3/", Properties: []string{"getctag"}, Depth: "1"},
		{Path: "/cal4/", Properties: []string{"displayname", "resourcetype"}, Depth: "0"},
	}

	optimized := processor.optimizeBatch(requests)

	if len(optimized) != 4 {
		t.Errorf("expected 4 optimized requests, got %d", len(optimized))
	}

	groupCounts := make(map[string]int)
	for _, req := range optimized {
		key := processor.createPropertyKey(req.Properties, req.Depth)
		groupCounts[key]++
	}

	if len(groupCounts) != 2 {
		t.Errorf("expected 2 property groups, got %d", len(groupCounts))
	}

	expectedKey1 := "0:displayname,resourcetype"
	expectedKey2 := "1:getctag"

	if groupCounts[expectedKey1] != 3 {
		t.Errorf("expected 3 requests in group '%s', got %d", expectedKey1, groupCounts[expectedKey1])
	}

	if groupCounts[expectedKey2] != 1 {
		t.Errorf("expected 1 request in group '%s', got %d", expectedKey2, groupCounts[expectedKey2])
	}
}

func TestBatchProcessor_SplitIntoBatches(t *testing.T) {
	client := NewClient("test", "test")
	processor := NewBatchProcessor(client, 3, 30*time.Second, 3)

	requests := make([]BatchRequest, 10)
	for i := range requests {
		requests[i] = BatchRequest{
			Path:       "/calendar" + string(rune('1'+i)) + "/",
			Properties: []string{"displayname"},
			Depth:      "0",
		}
	}

	batches := processor.splitIntoBatches(requests, 3)

	expectedBatches := 4
	if len(batches) != expectedBatches {
		t.Errorf("expected %d batches, got %d", expectedBatches, len(batches))
	}

	if len(batches[0]) != 3 {
		t.Errorf("expected first batch to have 3 requests, got %d", len(batches[0]))
	}

	if len(batches[3]) != 1 {
		t.Errorf("expected last batch to have 1 request, got %d", len(batches[3]))
	}
}

func TestBatchProcessor_WithCache(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>` + r.URL.Path + `</D:href>
    <D:propstat>
      <D:prop><D:displayname>Test</D:displayname></D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClientWithOptions("test", "test", WithCache(time.Minute, 100))
	client.baseURL = server.URL

	processor := NewBatchProcessor(client, 2, 30*time.Second, 2)

	requests := []BatchRequest{
		{Path: "/calendar/", Properties: []string{"displayname"}, Depth: "0"},
	}

	_, err := processor.ExecuteBatch(context.Background(), requests)
	if err != nil {
		t.Fatalf("first ExecuteBatch failed: %v", err)
	}

	_, err = processor.ExecuteBatch(context.Background(), requests)
	if err != nil {
		t.Fatalf("second ExecuteBatch failed: %v", err)
	}

	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request due to caching, got %d", requestCount)
	}
}

func TestCalendarBatchProcessor_BatchDiscoverCalendars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:A="http://apple.com/ns/ical/">
  <D:response>
    <D:href>/calendars/home/calendar1/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Personal</D:displayname>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <A:calendar-color>#FF0000FF</A:calendar-color>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/calendars/home/calendar2/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>Work</D:displayname>
        <D:resourcetype>
          <D:collection/>
          <C:calendar/>
        </D:resourcetype>
        <A:calendar-color>#00FF00FF</A:calendar-color>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	processor := client.NewCalendarBatchProcessor()

	homeSetPaths := []string{"/calendars/home/"}

	calendars, err := processor.BatchDiscoverCalendars(context.Background(), homeSetPaths)
	if err != nil {
		t.Fatalf("BatchDiscoverCalendars failed: %v", err)
	}

	if len(calendars) != 2 {
		t.Errorf("expected 2 calendars, got %d", len(calendars))
	}

	expectedNames := map[string]bool{"Personal": false, "Work": false}
	for _, cal := range calendars {
		if _, exists := expectedNames[cal.DisplayName]; exists {
			expectedNames[cal.DisplayName] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected calendar '%s' not found", name)
		}
	}
}

func TestBatchProcessor_EmptyRequests(t *testing.T) {
	client := NewClient("test", "test")
	processor := NewBatchProcessor(client, 10, 30*time.Second, 3)

	responses, err := processor.ExecuteBatch(context.Background(), []BatchRequest{})
	if err != nil {
		t.Errorf("ExecuteBatch with empty requests should not error, got: %v", err)
	}

	if responses != nil {
		t.Errorf("expected nil responses for empty request batch, got: %v", responses)
	}
}

func TestBatchProcessor_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(207)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/test/</D:href>
    <D:propstat>
      <D:prop><D:displayname>Test</D:displayname></D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClient("test", "test")
	client.baseURL = server.URL

	processor := NewBatchProcessor(client, 10, 50*time.Millisecond, 3)

	requests := []BatchRequest{
		{Path: "/test/", Properties: []string{"displayname"}, Depth: "0"},
	}

	responses, err := processor.ExecuteBatch(context.Background(), requests)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if len(responses) != 1 {
		t.Errorf("expected 1 response, got %d", len(responses))
	}

	if responses[0].Error == nil {
		t.Error("expected timeout error, got nil")
	}
}
