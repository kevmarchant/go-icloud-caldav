package caldav

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BatchQueryRequest represents a request for querying a single calendar in a batch operation.
type BatchQueryRequest struct {
	CalendarPath string
	Query        CalendarQuery
}

// BatchQueryResult contains the result from querying a single calendar.
type BatchQueryResult struct {
	CalendarPath string
	Objects      []CalendarObject
	Error        error
}

// BatchQueryConfig configures batch query operations.
type BatchQueryConfig struct {
	MaxConcurrency int
	Timeout        time.Duration
}

// DefaultBatchQueryConfig returns a default configuration for batch queries.
func DefaultBatchQueryConfig() *BatchQueryConfig {
	return &BatchQueryConfig{
		MaxConcurrency: 5,
		Timeout:        30 * time.Second,
	}
}

// QueryCalendarsParallel performs parallel queries on multiple calendars.
// It uses a worker pool to limit concurrency and returns results for all calendars.
func (c *CalDAVClient) QueryCalendarsParallel(ctx context.Context, requests []BatchQueryRequest, config *BatchQueryConfig) []BatchQueryResult {
	if config == nil {
		config = DefaultBatchQueryConfig()
	}

	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 1
	}

	numRequests := len(requests)
	if numRequests == 0 {
		return []BatchQueryResult{}
	}

	if config.MaxConcurrency > numRequests {
		config.MaxConcurrency = numRequests
	}

	results := make([]BatchQueryResult, numRequests)
	workChan := make(chan int, numRequests)
	var wg sync.WaitGroup

	c.logger.Debug("Starting parallel query for %d calendars with %d workers", numRequests, config.MaxConcurrency)

	for i := 0; i < config.MaxConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.logger.Debug("Worker %d started", workerID)

			for idx := range workChan {
				req := requests[idx]
				c.logger.Debug("Worker %d processing calendar %s", workerID, req.CalendarPath)

				queryCtx := ctx
				var cancel context.CancelFunc
				if config.Timeout > 0 {
					queryCtx, cancel = context.WithTimeout(ctx, config.Timeout)
				}

				objects, err := c.QueryCalendar(queryCtx, req.CalendarPath, req.Query)

				if cancel != nil {
					cancel()
				}

				results[idx] = BatchQueryResult{
					CalendarPath: req.CalendarPath,
					Objects:      objects,
					Error:        err,
				}

				if err != nil {
					c.logger.Error("Worker %d failed to query calendar %s: %v", workerID, req.CalendarPath, err)
				} else {
					c.logger.Debug("Worker %d successfully queried calendar %s, got %d objects", workerID, req.CalendarPath, len(objects))
				}
			}

			c.logger.Debug("Worker %d finished", workerID)
		}(i)
	}

	for i := range requests {
		workChan <- i
	}
	close(workChan)

	wg.Wait()

	c.logger.Debug("Parallel query completed for %d calendars", numRequests)

	return results
}

// GetRecentEventsParallel retrieves recent events from multiple calendars in parallel.
func (c *CalDAVClient) GetRecentEventsParallel(ctx context.Context, calendarPaths []string, days int, config *BatchQueryConfig) []BatchQueryResult {
	now := time.Now()
	startTime := now.AddDate(0, 0, -days)
	endTime := now.AddDate(0, 0, days)

	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		TimeRange: &TimeRange{
			Start: startTime,
			End:   endTime,
		},
	}

	requests := make([]BatchQueryRequest, len(calendarPaths))
	for i, path := range calendarPaths {
		requests[i] = BatchQueryRequest{
			CalendarPath: path,
			Query:        query,
		}
	}

	return c.QueryCalendarsParallel(ctx, requests, config)
}

// GetEventsByTimeRangeParallel retrieves events within a time range from multiple calendars in parallel.
func (c *CalDAVClient) GetEventsByTimeRangeParallel(ctx context.Context, calendarPaths []string, start, end time.Time, config *BatchQueryConfig) []BatchQueryResult {
	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		TimeRange: &TimeRange{
			Start: start,
			End:   end,
		},
	}

	requests := make([]BatchQueryRequest, len(calendarPaths))
	for i, path := range calendarPaths {
		requests[i] = BatchQueryRequest{
			CalendarPath: path,
			Query:        query,
		}
	}

	return c.QueryCalendarsParallel(ctx, requests, config)
}

// AggregateResults combines results from multiple calendars into a single slice.
// It returns all successful results and a slice of errors from failed queries.
func AggregateResults(results []BatchQueryResult) ([]CalendarObject, []error) {
	var allObjects []CalendarObject
	var allErrors []error

	for _, result := range results {
		if result.Error != nil {
			allErrors = append(allErrors, fmt.Errorf("calendar %s: %w", result.CalendarPath, result.Error))
		} else {
			allObjects = append(allObjects, result.Objects...)
		}
	}

	return allObjects, allErrors
}

// CountObjectsInResults counts the total number of calendar objects across all results.
func CountObjectsInResults(results []BatchQueryResult) int {
	count := 0
	for _, result := range results {
		if result.Error == nil {
			count += len(result.Objects)
		}
	}
	return count
}

// FilterSuccessfulResults returns only the results that completed without errors.
func FilterSuccessfulResults(results []BatchQueryResult) []BatchQueryResult {
	var successful []BatchQueryResult
	for _, result := range results {
		if result.Error == nil {
			successful = append(successful, result)
		}
	}
	return successful
}

// FilterFailedResults returns only the results that had errors.
func FilterFailedResults(results []BatchQueryResult) []BatchQueryResult {
	var failed []BatchQueryResult
	for _, result := range results {
		if result.Error != nil {
			failed = append(failed, result)
		}
	}
	return failed
}
