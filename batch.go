package caldav

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type BatchRequest struct {
	Path       string
	Properties []string
	Depth      string
}

type BatchResponse struct {
	Path     string
	Response *MultiStatusResponse
	Error    error
}

type BatchProcessor struct {
	client     *CalDAVClient
	maxBatch   int
	timeout    time.Duration
	maxWorkers int
}

func NewBatchProcessor(client *CalDAVClient, maxBatch int, timeout time.Duration, maxWorkers int) *BatchProcessor {
	if maxBatch <= 0 {
		maxBatch = 10
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if maxWorkers <= 0 {
		maxWorkers = 5
	}

	return &BatchProcessor{
		client:     client,
		maxBatch:   maxBatch,
		timeout:    timeout,
		maxWorkers: maxWorkers,
	}
}

func (bp *BatchProcessor) ExecuteBatch(ctx context.Context, requests []BatchRequest) ([]BatchResponse, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	optimizedRequests := bp.optimizeBatch(requests)

	batches := bp.splitIntoBatches(optimizedRequests, bp.maxBatch)

	results := make([]BatchResponse, 0, len(requests))
	var mu sync.Mutex

	semaphore := make(chan struct{}, bp.maxWorkers)
	var wg sync.WaitGroup

	for _, batch := range batches {
		wg.Add(1)
		go func(batchRequests []BatchRequest) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			batchResults := bp.processBatch(ctx, batchRequests)

			mu.Lock()
			results = append(results, batchResults...)
			mu.Unlock()
		}(batch)
	}

	wg.Wait()

	return results, nil
}

func (bp *BatchProcessor) optimizeBatch(requests []BatchRequest) []BatchRequest {
	propertyGroups := make(map[string][]BatchRequest)

	for _, req := range requests {
		key := bp.createPropertyKey(req.Properties, req.Depth)
		propertyGroups[key] = append(propertyGroups[key], req)
	}

	optimized := make([]BatchRequest, 0, len(requests))
	for _, group := range propertyGroups {
		optimized = append(optimized, group...)
	}

	return optimized
}

func (bp *BatchProcessor) createPropertyKey(properties []string, depth string) string {
	if len(properties) == 0 {
		return depth
	}

	key := depth + ":"
	for i, prop := range properties {
		if i > 0 {
			key += ","
		}
		key += prop
	}
	return key
}

func (bp *BatchProcessor) splitIntoBatches(requests []BatchRequest, batchSize int) [][]BatchRequest {
	var batches [][]BatchRequest

	for i := 0; i < len(requests); i += batchSize {
		end := i + batchSize
		if end > len(requests) {
			end = len(requests)
		}
		batches = append(batches, requests[i:end])
	}

	return batches
}

func (bp *BatchProcessor) processBatch(ctx context.Context, requests []BatchRequest) []BatchResponse {
	results := make([]BatchResponse, len(requests))

	for i, req := range requests {
		batchCtx, cancel := context.WithTimeout(ctx, bp.timeout)

		xmlBody, err := buildPropfindXML(req.Properties)
		if err != nil {
			results[i] = BatchResponse{
				Path:  req.Path,
				Error: fmt.Errorf("building XML: %w", err),
			}
			cancel()
			continue
		}

		cacheOp := &CachedOperation{
			Operation: "batch-propfind",
			Path:      req.Path,
			Body:      xmlBody,
			TTL:       5 * time.Minute,
		}

		if cached, found := bp.client.getCachedResponse(batchCtx, cacheOp); found {
			if msResp, ok := cached.(*MultiStatusResponse); ok {
				bp.client.logger.Debug("Using cached batch PROPFIND for: %s", req.Path)
				results[i] = BatchResponse{
					Path:     req.Path,
					Response: msResp,
				}
				cancel()
				continue
			}
		}

		resp, err := bp.client.propfind(batchCtx, req.Path, req.Depth, xmlBody)
		if err != nil {
			results[i] = BatchResponse{
				Path:  req.Path,
				Error: fmt.Errorf("PROPFIND failed: %w", err),
			}
			cancel()
			continue
		}

		msResp, err := parseMultiStatusResponse(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			results[i] = BatchResponse{
				Path:  req.Path,
				Error: fmt.Errorf("parsing response: %w", err),
			}
		} else {
			bp.client.setCachedResponse(cacheOp, msResp)
			results[i] = BatchResponse{
				Path:     req.Path,
				Response: msResp,
			}
		}

		cancel()
	}

	return results
}

func (c *CalDAVClient) NewBatchProcessor(maxBatch int, timeout time.Duration, maxWorkers int) *BatchProcessor {
	return NewBatchProcessor(c, maxBatch, timeout, maxWorkers)
}

func (c *CalDAVClient) BatchPropfind(ctx context.Context, requests []BatchRequest) ([]BatchResponse, error) {
	processor := c.NewBatchProcessor(10, 30*time.Second, 5)
	return processor.ExecuteBatch(ctx, requests)
}

type CalendarBatchProcessor struct {
	*BatchProcessor
}

func (c *CalDAVClient) NewCalendarBatchProcessor() *CalendarBatchProcessor {
	processor := NewBatchProcessor(c, 10, 30*time.Second, 5)
	return &CalendarBatchProcessor{BatchProcessor: processor}
}

func (cbp *CalendarBatchProcessor) BatchDiscoverCalendars(ctx context.Context, homeSetPaths []string) ([]Calendar, error) {
	requests := make([]BatchRequest, len(homeSetPaths))

	calendarProps := []string{
		"displayname",
		"resourcetype",
		"calendar-description",
		"calendar-color",
		"supported-calendar-component-set",
		"getctag",
		"getetag",
		"calendar-timezone",
		"max-resource-size",
		"min-date-time",
		"max-date-time",
		"max-instances",
		"max-attendees-per-instance",
		"current-user-privilege-set",
		"source",
		"supported-report-set",
		"quota-used-bytes",
		"quota-available-bytes",
	}

	for i, path := range homeSetPaths {
		requests[i] = BatchRequest{
			Path:       path,
			Properties: calendarProps,
			Depth:      "1",
		}
	}

	responses, err := cbp.ExecuteBatch(ctx, requests)
	if err != nil {
		return nil, err
	}

	var allCalendars []Calendar
	for _, resp := range responses {
		if resp.Error != nil {
			cbp.client.logger.Error("Failed to discover calendars at %s: %v", resp.Path, resp.Error)
			continue
		}

		calendars := extractCalendarsFromResponse(resp.Response)
		allCalendars = append(allCalendars, calendars...)
	}

	return allCalendars, nil
}

type BatchStats struct {
	TotalRequests   int
	SuccessfulBatch int
	FailedBatch     int
	CacheHits       int
	TotalDuration   time.Duration
	AverageLatency  time.Duration
}

func (bp *BatchProcessor) GetStats() BatchStats {
	return BatchStats{}
}
