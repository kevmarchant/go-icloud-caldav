package caldav

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type CRUDOperation int

const (
	OpCreate CRUDOperation = iota
	OpUpdate
	OpDelete
)

type BatchCRUDRequest struct {
	Operation    CRUDOperation
	CalendarPath string
	Event        *CalendarObject
	EventPath    string
	ETag         string
	RequestID    string
}

type BatchCRUDResponse struct {
	RequestID  string
	Operation  CRUDOperation
	Success    bool
	StatusCode int
	ETag       string
	Error      error
	Duration   time.Duration
}

type CRUDBatchProcessor struct {
	client     *CalDAVClient
	maxBatch   int
	timeout    time.Duration
	maxWorkers int
	metrics    *BatchCRUDMetrics
}

type BatchCRUDMetrics struct {
	TotalRequests  int64
	SuccessfulOps  int64
	FailedOps      int64
	CreateOps      int64
	UpdateOps      int64
	DeleteOps      int64
	TotalDuration  int64
	FastestOp      int64
	SlowestOp      int64
	mu             sync.RWMutex
	operationTimes []time.Duration
}

func NewCRUDBatchProcessor(client *CalDAVClient, options ...func(*CRUDBatchProcessor)) *CRUDBatchProcessor {
	processor := &CRUDBatchProcessor{
		client:     client,
		maxBatch:   20,
		timeout:    30 * time.Second,
		maxWorkers: 10,
		metrics:    &BatchCRUDMetrics{},
	}

	for _, opt := range options {
		opt(processor)
	}

	return processor
}

func WithMaxBatch(size int) func(*CRUDBatchProcessor) {
	return func(p *CRUDBatchProcessor) {
		if size > 0 {
			p.maxBatch = size
		}
	}
}

func WithTimeout(timeout time.Duration) func(*CRUDBatchProcessor) {
	return func(p *CRUDBatchProcessor) {
		if timeout > 0 {
			p.timeout = timeout
		}
	}
}

func WithMaxWorkers(workers int) func(*CRUDBatchProcessor) {
	return func(p *CRUDBatchProcessor) {
		if workers > 0 {
			p.maxWorkers = workers
		}
	}
}

func (bp *CRUDBatchProcessor) ExecuteBatch(ctx context.Context, requests []BatchCRUDRequest) ([]BatchCRUDResponse, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		atomic.AddInt64(&bp.metrics.TotalDuration, int64(duration))
	}()

	atomic.AddInt64(&bp.metrics.TotalRequests, int64(len(requests)))

	responses := make([]BatchCRUDResponse, len(requests))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, bp.maxWorkers)

	for i, req := range requests {
		wg.Add(1)
		go func(index int, request BatchCRUDRequest) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			response := bp.processRequest(ctx, request)
			responses[index] = response

			bp.updateMetrics(response)
		}(i, req)
	}

	wg.Wait()

	return responses, nil
}

func (bp *CRUDBatchProcessor) processRequest(ctx context.Context, req BatchCRUDRequest) BatchCRUDResponse {
	reqCtx, cancel := context.WithTimeout(ctx, bp.timeout)
	defer cancel()

	startTime := time.Now()
	response := BatchCRUDResponse{
		RequestID: req.RequestID,
		Operation: req.Operation,
	}

	var err error
	var statusCode int
	var etag string

	switch req.Operation {
	case OpCreate:
		atomic.AddInt64(&bp.metrics.CreateOps, 1)
		statusCode, etag, err = bp.executeCreate(reqCtx, req)
	case OpUpdate:
		atomic.AddInt64(&bp.metrics.UpdateOps, 1)
		statusCode, etag, err = bp.executeUpdate(reqCtx, req)
	case OpDelete:
		atomic.AddInt64(&bp.metrics.DeleteOps, 1)
		statusCode, err = bp.executeDelete(reqCtx, req)
	}

	response.Duration = time.Since(startTime)
	response.StatusCode = statusCode
	response.ETag = etag
	response.Error = err
	response.Success = err == nil

	return response
}

func (bp *CRUDBatchProcessor) executeCreate(ctx context.Context, req BatchCRUDRequest) (int, string, error) {
	if req.Event == nil {
		return 0, "", fmt.Errorf("event is required for create operation")
	}

	if err := validateEventForCreation(req.Event); err != nil {
		return 0, "", fmt.Errorf("validation failed: %w", err)
	}

	if req.Event.UID == "" {
		req.Event.UID = generateUID()
	}

	icalData, err := generateICalendar(req.Event)
	if err != nil {
		return 0, "", fmt.Errorf("generating iCalendar: %w", err)
	}

	eventURL := buildEventURL(bp.client.baseURL, req.CalendarPath, req.Event.UID)

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", eventURL, strings.NewReader(icalData))
	if err != nil {
		return 0, "", err
	}

	httpReq.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	httpReq.Header.Set("If-None-Match", "*")
	httpReq.SetBasicAuth(bp.client.username, bp.client.password)

	resp, err := bp.client.httpClient.Do(httpReq)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	etag := resp.Header.Get("ETag")

	if resp.StatusCode == http.StatusCreated {
		return resp.StatusCode, etag, nil
	}

	if resp.StatusCode == http.StatusPreconditionFailed {
		return resp.StatusCode, "", fmt.Errorf("event already exists")
	}

	return resp.StatusCode, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func (bp *CRUDBatchProcessor) executeUpdate(ctx context.Context, req BatchCRUDRequest) (int, string, error) {
	if req.Event == nil {
		return 0, "", fmt.Errorf("event is required for update operation")
	}

	if req.Event.UID == "" {
		return 0, "", fmt.Errorf("UID is required for update operation")
	}

	if err := validateEventForUpdate(req.Event); err != nil {
		return 0, "", fmt.Errorf("validation failed: %w", err)
	}

	icalData, err := generateICalendar(req.Event)
	if err != nil {
		return 0, "", fmt.Errorf("generating iCalendar: %w", err)
	}

	eventURL := buildEventURL(bp.client.baseURL, req.CalendarPath, req.Event.UID)

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", eventURL, strings.NewReader(icalData))
	if err != nil {
		return 0, "", err
	}

	httpReq.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	if req.ETag != "" {
		httpReq.Header.Set("If-Match", req.ETag)
	}
	httpReq.SetBasicAuth(bp.client.username, bp.client.password)

	resp, err := bp.client.httpClient.Do(httpReq)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	etag := resp.Header.Get("ETag")

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return resp.StatusCode, etag, nil
	}

	if resp.StatusCode == http.StatusPreconditionFailed {
		return resp.StatusCode, "", fmt.Errorf("etag mismatch - concurrent modification detected")
	}

	return resp.StatusCode, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func (bp *CRUDBatchProcessor) executeDelete(ctx context.Context, req BatchCRUDRequest) (int, error) {
	if req.EventPath == "" {
		return 0, fmt.Errorf("event path is required for delete operation")
	}

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", bp.client.baseURL+req.EventPath, nil)
	if err != nil {
		return 0, err
	}

	if req.ETag != "" {
		httpReq.Header.Set("If-Match", req.ETag)
	}
	httpReq.SetBasicAuth(bp.client.username, bp.client.password)

	resp, err := bp.client.httpClient.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return resp.StatusCode, nil
	}

	if resp.StatusCode == http.StatusPreconditionFailed {
		return resp.StatusCode, fmt.Errorf("etag mismatch - concurrent modification detected")
	}

	return resp.StatusCode, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func (bp *CRUDBatchProcessor) updateMetrics(response BatchCRUDResponse) {
	if response.Success {
		atomic.AddInt64(&bp.metrics.SuccessfulOps, 1)
	} else {
		atomic.AddInt64(&bp.metrics.FailedOps, 1)
	}

	durationNanos := int64(response.Duration)

	bp.metrics.mu.Lock()
	bp.metrics.operationTimes = append(bp.metrics.operationTimes, response.Duration)
	bp.metrics.mu.Unlock()

	for {
		fastest := atomic.LoadInt64(&bp.metrics.FastestOp)
		if fastest == 0 || durationNanos < fastest {
			if atomic.CompareAndSwapInt64(&bp.metrics.FastestOp, fastest, durationNanos) {
				break
			}
		} else {
			break
		}
	}

	for {
		slowest := atomic.LoadInt64(&bp.metrics.SlowestOp)
		if durationNanos > slowest {
			if atomic.CompareAndSwapInt64(&bp.metrics.SlowestOp, slowest, durationNanos) {
				break
			}
		} else {
			break
		}
	}
}

func (bp *CRUDBatchProcessor) GetMetrics() *BatchCRUDMetrics {
	bp.metrics.mu.RLock()
	defer bp.metrics.mu.RUnlock()

	metrics := &BatchCRUDMetrics{
		TotalRequests:  atomic.LoadInt64(&bp.metrics.TotalRequests),
		SuccessfulOps:  atomic.LoadInt64(&bp.metrics.SuccessfulOps),
		FailedOps:      atomic.LoadInt64(&bp.metrics.FailedOps),
		CreateOps:      atomic.LoadInt64(&bp.metrics.CreateOps),
		UpdateOps:      atomic.LoadInt64(&bp.metrics.UpdateOps),
		DeleteOps:      atomic.LoadInt64(&bp.metrics.DeleteOps),
		TotalDuration:  atomic.LoadInt64(&bp.metrics.TotalDuration),
		FastestOp:      atomic.LoadInt64(&bp.metrics.FastestOp),
		SlowestOp:      atomic.LoadInt64(&bp.metrics.SlowestOp),
		operationTimes: make([]time.Duration, len(bp.metrics.operationTimes)),
	}

	copy(metrics.operationTimes, bp.metrics.operationTimes)
	return metrics
}

func (m *BatchCRUDMetrics) AverageDuration() time.Duration {
	if m.TotalRequests == 0 {
		return 0
	}
	return time.Duration(m.TotalDuration / m.TotalRequests)
}

func (m *BatchCRUDMetrics) SuccessRate() float64 {
	if m.TotalRequests == 0 {
		return 0
	}
	return float64(m.SuccessfulOps) / float64(m.TotalRequests) * 100
}

func (m *BatchCRUDMetrics) FastestDuration() time.Duration {
	return time.Duration(m.FastestOp)
}

func (m *BatchCRUDMetrics) SlowestDuration() time.Duration {
	return time.Duration(m.SlowestOp)
}

func (c *CalDAVClient) NewCRUDBatchProcessor(options ...func(*CRUDBatchProcessor)) *CRUDBatchProcessor {
	return NewCRUDBatchProcessor(c, options...)
}

func (c *CalDAVClient) BatchCreateEvents(ctx context.Context, calendarPath string, events []*CalendarObject) ([]BatchCRUDResponse, error) {
	processor := c.NewCRUDBatchProcessor()
	requests := make([]BatchCRUDRequest, len(events))

	for i, event := range events {
		requests[i] = BatchCRUDRequest{
			Operation:    OpCreate,
			CalendarPath: calendarPath,
			Event:        event,
			RequestID:    fmt.Sprintf("create-%d", i),
		}
	}

	return processor.ExecuteBatch(ctx, requests)
}

func (c *CalDAVClient) BatchUpdateEvents(ctx context.Context, calendarPath string, updates []struct {
	Event *CalendarObject
	ETag  string
}) ([]BatchCRUDResponse, error) {
	processor := c.NewCRUDBatchProcessor()
	requests := make([]BatchCRUDRequest, len(updates))

	for i, update := range updates {
		requests[i] = BatchCRUDRequest{
			Operation:    OpUpdate,
			CalendarPath: calendarPath,
			Event:        update.Event,
			ETag:         update.ETag,
			RequestID:    fmt.Sprintf("update-%d", i),
		}
	}

	return processor.ExecuteBatch(ctx, requests)
}

func (c *CalDAVClient) BatchDeleteEvents(ctx context.Context, eventPaths []string) ([]BatchCRUDResponse, error) {
	processor := c.NewCRUDBatchProcessor()
	requests := make([]BatchCRUDRequest, len(eventPaths))

	for i, path := range eventPaths {
		requests[i] = BatchCRUDRequest{
			Operation: OpDelete,
			EventPath: path,
			RequestID: fmt.Sprintf("delete-%d", i),
		}
	}

	return processor.ExecuteBatch(ctx, requests)
}
