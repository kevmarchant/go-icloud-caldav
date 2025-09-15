package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCRUDBatchProcessor_ExecuteBatch(t *testing.T) {
	var createCount, updateCount, deleteCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			if r.Header.Get("If-None-Match") == "*" {
				atomic.AddInt32(&createCount, 1)
				w.Header().Set("ETag", fmt.Sprintf("etag-%d", createCount))
				w.WriteHeader(http.StatusCreated)
			} else {
				atomic.AddInt32(&updateCount, 1)
				w.Header().Set("ETag", fmt.Sprintf("etag-updated-%d", updateCount))
				w.WriteHeader(http.StatusNoContent)
			}
		case "DELETE":
			atomic.AddInt32(&deleteCount, 1)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	processor := NewCRUDBatchProcessor(client, WithMaxWorkers(2), WithTimeout(5*time.Second))

	requests := []BatchCRUDRequest{
		{
			Operation:    OpCreate,
			CalendarPath: "/calendars/test/",
			Event: &CalendarObject{
				UID:       "test-1",
				Summary:   "Test Event 1",
				StartTime: timePtr(time.Now()),
				EndTime:   timePtr(time.Now().Add(time.Hour)),
			},
			RequestID: "create-1",
		},
		{
			Operation:    OpUpdate,
			CalendarPath: "/calendars/test/",
			Event: &CalendarObject{
				UID:       "test-2",
				Summary:   "Updated Event",
				StartTime: timePtr(time.Now()),
				EndTime:   timePtr(time.Now().Add(time.Hour)),
			},
			ETag:      "existing-etag",
			RequestID: "update-1",
		},
		{
			Operation: OpDelete,
			EventPath: "/calendars/test/test-3.ics",
			RequestID: "delete-1",
		},
	}

	ctx := context.Background()
	responses, err := processor.ExecuteBatch(ctx, requests)

	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	if atomic.LoadInt32(&createCount) != 1 {
		t.Errorf("Expected 1 create operation, got %d", createCount)
	}

	if atomic.LoadInt32(&updateCount) != 1 {
		t.Errorf("Expected 1 update operation, got %d", updateCount)
	}

	if atomic.LoadInt32(&deleteCount) != 1 {
		t.Errorf("Expected 1 delete operation, got %d", deleteCount)
	}

	for i, resp := range responses {
		if !resp.Success {
			t.Errorf("Request %d failed: %v", i, resp.Error)
		}
		if resp.RequestID == "" {
			t.Errorf("Request %d missing RequestID", i)
		}
		if resp.Duration == 0 {
			t.Errorf("Request %d has zero duration", i)
		}
	}
}

func TestBatchCRUDMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)

		if strings.Contains(r.URL.Path, "fail") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if r.Method == "PUT" && r.Header.Get("If-None-Match") == "*" {
			w.Header().Set("ETag", "new-etag")
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	processor := NewCRUDBatchProcessor(client)

	requests := []BatchCRUDRequest{
		{
			Operation:    OpCreate,
			CalendarPath: "/calendars/test/",
			Event: &CalendarObject{
				UID:       "success-1",
				Summary:   "Success Event",
				StartTime: timePtr(time.Now()),
				EndTime:   timePtr(time.Now().Add(time.Hour)),
			},
			RequestID: "create-success",
		},
		{
			Operation:    OpCreate,
			CalendarPath: "/calendars/test/",
			Event: &CalendarObject{
				UID:       "fail-1",
				Summary:   "Fail Event",
				StartTime: timePtr(time.Now()),
				EndTime:   timePtr(time.Now().Add(time.Hour)),
			},
			RequestID: "create-fail",
		},
	}

	ctx := context.Background()
	_, err := processor.ExecuteBatch(ctx, requests)
	if err != nil {
		t.Fatalf("ExecuteBatch failed: %v", err)
	}

	metrics := processor.GetMetrics()

	if metrics.TotalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.SuccessfulOps != 1 {
		t.Errorf("Expected 1 successful operation, got %d", metrics.SuccessfulOps)
	}

	if metrics.FailedOps != 1 {
		t.Errorf("Expected 1 failed operation, got %d", metrics.FailedOps)
	}

	if metrics.CreateOps != 2 {
		t.Errorf("Expected 2 create operations, got %d", metrics.CreateOps)
	}

	if metrics.SuccessRate() != 50.0 {
		t.Errorf("Expected 50%% success rate, got %.2f%%", metrics.SuccessRate())
	}

	if metrics.AverageDuration() == 0 {
		t.Error("Average duration should not be zero")
	}

	if metrics.FastestDuration() == 0 {
		t.Error("Fastest duration should not be zero")
	}

	if metrics.SlowestDuration() == 0 {
		t.Error("Slowest duration should not be zero")
	}

	if metrics.FastestDuration() > metrics.SlowestDuration() {
		t.Error("Fastest duration should be less than or equal to slowest")
	}
}

func TestBatchCreateEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.Header.Get("If-None-Match") != "*" {
			t.Error("Expected If-None-Match: * header")
		}
		w.Header().Set("ETag", "created-etag")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	events := []*CalendarObject{
		{
			Summary:   "Event 1",
			StartTime: timePtr(time.Now()),
			EndTime:   timePtr(time.Now().Add(time.Hour)),
		},
		{
			Summary:   "Event 2",
			StartTime: timePtr(time.Now().Add(2 * time.Hour)),
			EndTime:   timePtr(time.Now().Add(3 * time.Hour)),
		},
	}

	ctx := context.Background()
	responses, err := client.BatchCreateEvents(ctx, "/calendars/test/", events)

	if err != nil {
		t.Fatalf("BatchCreateEvents failed: %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(responses))
	}

	for i, resp := range responses {
		if !resp.Success {
			t.Errorf("Event %d creation failed: %v", i, resp.Error)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Event %d: expected status 201, got %d", i, resp.StatusCode)
		}
		if resp.ETag != "created-etag" {
			t.Errorf("Event %d: expected ETag 'created-etag', got '%s'", i, resp.ETag)
		}
	}
}

func TestBatchUpdateEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		if r.Header.Get("If-Match") == "" {
			t.Error("Expected If-Match header")
		}
		w.Header().Set("ETag", "updated-etag")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	updates := []struct {
		Event *CalendarObject
		ETag  string
	}{
		{
			Event: &CalendarObject{
				UID:       "update-1",
				Summary:   "Updated Event 1",
				StartTime: timePtr(time.Now()),
				EndTime:   timePtr(time.Now().Add(time.Hour)),
			},
			ETag: "etag-1",
		},
		{
			Event: &CalendarObject{
				UID:       "update-2",
				Summary:   "Updated Event 2",
				StartTime: timePtr(time.Now()),
				EndTime:   timePtr(time.Now().Add(2 * time.Hour)),
			},
			ETag: "etag-2",
		},
	}

	ctx := context.Background()
	responses, err := client.BatchUpdateEvents(ctx, "/calendars/test/", updates)

	if err != nil {
		t.Fatalf("BatchUpdateEvents failed: %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(responses))
	}

	for i, resp := range responses {
		if !resp.Success {
			t.Errorf("Event %d update failed: %v", i, resp.Error)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Event %d: expected status 204, got %d", i, resp.StatusCode)
		}
	}
}

func TestBatchDeleteEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &CalDAVClient{
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	eventPaths := []string{
		"/calendars/test/event1.ics",
		"/calendars/test/event2.ics",
		"/calendars/test/event3.ics",
	}

	ctx := context.Background()
	responses, err := client.BatchDeleteEvents(ctx, eventPaths)

	if err != nil {
		t.Fatalf("BatchDeleteEvents failed: %v", err)
	}

	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	for i, resp := range responses {
		if !resp.Success {
			t.Errorf("Event %d deletion failed: %v", i, resp.Error)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Event %d: expected status 204, got %d", i, resp.StatusCode)
		}
	}
}

func TestBatchCRUDValidation(t *testing.T) {
	client := &CalDAVClient{
		baseURL:    "http://test",
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	processor := NewCRUDBatchProcessor(client)

	t.Run("CreateWithoutEvent", func(t *testing.T) {
		requests := []BatchCRUDRequest{
			{
				Operation:    OpCreate,
				CalendarPath: "/calendars/test/",
				Event:        nil,
				RequestID:    "invalid-create",
			},
		}

		ctx := context.Background()
		responses, err := processor.ExecuteBatch(ctx, requests)

		if err != nil {
			t.Fatalf("ExecuteBatch failed: %v", err)
		}

		if len(responses) != 1 {
			t.Fatalf("Expected 1 response, got %d", len(responses))
		}

		if responses[0].Success {
			t.Error("Expected failure for create without event")
		}

		if responses[0].Error == nil {
			t.Error("Expected error for create without event")
		}
	})

	t.Run("UpdateWithoutUID", func(t *testing.T) {
		requests := []BatchCRUDRequest{
			{
				Operation:    OpUpdate,
				CalendarPath: "/calendars/test/",
				Event: &CalendarObject{
					Summary:   "No UID",
					StartTime: timePtr(time.Now()),
					EndTime:   timePtr(time.Now().Add(time.Hour)),
				},
				RequestID: "invalid-update",
			},
		}

		ctx := context.Background()
		responses, err := processor.ExecuteBatch(ctx, requests)

		if err != nil {
			t.Fatalf("ExecuteBatch failed: %v", err)
		}

		if len(responses) != 1 {
			t.Fatalf("Expected 1 response, got %d", len(responses))
		}

		if responses[0].Success {
			t.Error("Expected failure for update without UID")
		}
	})

	t.Run("DeleteWithoutPath", func(t *testing.T) {
		requests := []BatchCRUDRequest{
			{
				Operation: OpDelete,
				EventPath: "",
				RequestID: "invalid-delete",
			},
		}

		ctx := context.Background()
		responses, err := processor.ExecuteBatch(ctx, requests)

		if err != nil {
			t.Fatalf("ExecuteBatch failed: %v", err)
		}

		if len(responses) != 1 {
			t.Fatalf("Expected 1 response, got %d", len(responses))
		}

		if responses[0].Success {
			t.Error("Expected failure for delete without path")
		}
	})
}

func TestBatchProcessorOptions(t *testing.T) {
	client := &CalDAVClient{
		baseURL:    "http://test",
		httpClient: http.DefaultClient,
		logger:     &noopLogger{},
	}

	t.Run("DefaultOptions", func(t *testing.T) {
		processor := NewCRUDBatchProcessor(client)

		if processor.maxBatch != 20 {
			t.Errorf("Expected default maxBatch 20, got %d", processor.maxBatch)
		}

		if processor.timeout != 30*time.Second {
			t.Errorf("Expected default timeout 30s, got %v", processor.timeout)
		}

		if processor.maxWorkers != 10 {
			t.Errorf("Expected default maxWorkers 10, got %d", processor.maxWorkers)
		}
	})

	t.Run("CustomOptions", func(t *testing.T) {
		processor := NewCRUDBatchProcessor(
			client,
			WithMaxBatch(50),
			WithTimeout(60*time.Second),
			WithMaxWorkers(20),
		)

		if processor.maxBatch != 50 {
			t.Errorf("Expected maxBatch 50, got %d", processor.maxBatch)
		}

		if processor.timeout != 60*time.Second {
			t.Errorf("Expected timeout 60s, got %v", processor.timeout)
		}

		if processor.maxWorkers != 20 {
			t.Errorf("Expected maxWorkers 20, got %d", processor.maxWorkers)
		}
	})
}
