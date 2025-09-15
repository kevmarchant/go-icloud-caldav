package caldav

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type syncMultiStatus struct {
	XMLName   xml.Name      `xml:"multistatus"`
	SyncToken string        `xml:"sync-token"`
	Responses []xmlResponse `xml:"response"`
}

type SyncToken struct {
	Token     string
	Timestamp time.Time
	Valid     bool
}

type SyncRequest struct {
	CalendarURL string
	SyncToken   string
	SyncLevel   int
	Properties  []string
	Limit       int
}

type SyncResponse struct {
	SyncToken    string
	Changes      []SyncChange
	MoreToSync   bool
	TotalChanges int
}

type SyncChange struct {
	Href         string
	Status       int
	ETag         string
	CalendarData string
	Deleted      bool
	Properties   map[string]string
}

type SyncChangeType int

// ETag cache types for conditional requests
type ETagCache struct {
	mu      sync.RWMutex
	entries map[string]*ETagEntry
	maxAge  time.Duration
}

type ETagEntry struct {
	ETag         string
	Content      []byte
	LastModified time.Time
	CachedAt     time.Time
	Size         int64
}

// Prefer header for optimizing responses
type PreferHeader struct {
	ReturnMinimal        bool
	ReturnRepresentation bool
	Wait                 *time.Duration
	HandlingStrict       bool
	HandlingLenient      bool
	RespondAsync         bool
	DepthNoroot          bool
	MaxResults           *int
}

// Batch operation types
type BatchOperation struct {
	Method      string
	Path        string
	Headers     map[string]string
	Body        []byte
	ETag        string
	IfMatch     string
	IfNoneMatch string
}

type BatchResult struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	ETag       string
	Error      error
}

// Delta sync types
type DeltaSyncState struct {
	SyncToken      string
	LastSync       time.Time
	Resources      map[string]*DeltaResource
	PendingDeletes []string
}

type DeltaResource struct {
	Href         string
	ETag         string
	LastModified time.Time
	ContentType  string
	Size         int64
	Checksum     string
}

const (
	SyncChangeTypeNew SyncChangeType = iota
	SyncChangeTypeModified
	SyncChangeTypeDeleted
)

func (c *SyncChange) ChangeType() SyncChangeType {
	if c.Deleted {
		return SyncChangeTypeDeleted
	}
	if c.CalendarData != "" && c.ETag != "" {
		return SyncChangeTypeModified
	}
	return SyncChangeTypeNew
}

func (c *CalDAVClient) SyncCalendar(ctx context.Context, req *SyncRequest) (*SyncResponse, error) {
	if req.CalendarURL == "" {
		return nil, newTypedError("SyncCalendar", ErrorTypeValidation, "calendar URL is required for sync", nil)
	}

	xmlBody := buildSyncCollectionXML(req)

	resp, err := c.report(ctx, req.CalendarURL, []byte(xmlBody))
	if err != nil {
		return nil, wrapErrorWithType("SyncCalendar", ErrorTypeNetwork, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		return nil, newTypedError("SyncCalendar", ErrorTypeServer, "unexpected status code", nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("SyncCalendar", ErrorTypeNetwork, err)
	}

	return parseSyncResponse(body)
}

func (c *CalDAVClient) InitialSync(ctx context.Context, calendarURL string) (*SyncResponse, error) {
	return c.SyncCalendar(ctx, &SyncRequest{
		CalendarURL: calendarURL,
		SyncToken:   "",
		SyncLevel:   1,
		Properties: []string{
			"getetag",
			"calendar-data",
		},
	})
}

func (c *CalDAVClient) IncrementalSync(ctx context.Context, calendarURL string, syncToken string) (*SyncResponse, error) {
	if syncToken == "" {
		return nil, newTypedError("IncrementalSync", ErrorTypeValidation, "sync token is required for incremental sync", nil)
	}

	return c.SyncCalendar(ctx, &SyncRequest{
		CalendarURL: calendarURL,
		SyncToken:   syncToken,
		SyncLevel:   1,
		Properties: []string{
			"getetag",
			"calendar-data",
		},
	})
}

func (c *CalDAVClient) SyncAllCalendars(ctx context.Context, syncTokens map[string]string) (map[string]*SyncResponse, error) {
	return c.SyncAllCalendarsWithWorkers(ctx, syncTokens, 5)
}

type syncJob struct {
	calendar  Calendar
	syncToken string
}

type syncResult struct {
	href     string
	response *SyncResponse
	err      error
}

func (c *CalDAVClient) SyncAllCalendarsWithWorkers(ctx context.Context, syncTokens map[string]string, maxWorkers int) (map[string]*SyncResponse, error) {
	calendars, err := c.DiscoverCalendars(ctx)
	if err != nil {
		return nil, wrapErrorWithType("SyncAllCalendarsWithWorkers", ErrorTypeInvalidRequest, err)
	}

	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	if maxWorkers > len(calendars) {
		maxWorkers = len(calendars)
	}

	jobs := make(chan syncJob, len(calendars))
	results := make(chan syncResult, len(calendars))

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < maxWorkers; i++ {
		go func() {
			for job := range jobs {
				select {
				case <-workerCtx.Done():
					results <- syncResult{
						href: job.calendar.Href,
						err:  workerCtx.Err(),
					}
					return
				default:
				}

				var syncResp *SyncResponse
				var err error

				if job.syncToken != "" {
					syncResp, err = c.IncrementalSync(workerCtx, job.calendar.Href, job.syncToken)
					if err != nil {
						syncResp, err = c.InitialSync(workerCtx, job.calendar.Href)
					}
				} else {
					syncResp, err = c.InitialSync(workerCtx, job.calendar.Href)
				}

				if err != nil && c.logger != nil {
					c.logger.Debug("Error syncing calendar", "name", job.calendar.DisplayName, "error", err)
				}

				results <- syncResult{
					href:     job.calendar.Href,
					response: syncResp,
					err:      err,
				}
			}
		}()
	}

	for _, cal := range calendars {
		token := syncTokens[cal.Href]
		jobs <- syncJob{
			calendar:  cal,
			syncToken: token,
		}
	}
	close(jobs)

	syncResults := make(map[string]*SyncResponse)
	for i := 0; i < len(calendars); i++ {
		result := <-results
		if result.err == nil && result.response != nil {
			syncResults[result.href] = result.response
		}
	}

	return syncResults, nil
}

func buildSyncCollectionXML(req *SyncRequest) string {
	var buf strings.Builder

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	if req.SyncToken != "" {
		buf.WriteString(`<D:sync-token>`)
		buf.WriteString(xmlEscape(req.SyncToken))
		buf.WriteString(`</D:sync-token>`)
	} else {
		buf.WriteString(`<D:sync-token/>`)
	}

	buf.WriteString(`<D:sync-level>`)
	if req.SyncLevel > 0 {
		buf.WriteString("1")
	} else {
		buf.WriteString("infinite")
	}
	buf.WriteString(`</D:sync-level>`)

	if req.Limit > 0 {
		buf.WriteString(`<D:limit><D:nresults>`)
		buf.WriteString(intToString(req.Limit))
		buf.WriteString(`</D:nresults></D:limit>`)
	}

	buf.WriteString(`<D:prop>`)
	for _, prop := range req.Properties {
		switch prop {
		case "getetag":
			buf.WriteString(`<D:getetag/>`)
		case "calendar-data":
			buf.WriteString(`<C:calendar-data/>`)
		case "getcontenttype":
			buf.WriteString(`<D:getcontenttype/>`)
		case "displayname":
			buf.WriteString(`<D:displayname/>`)
		default:
			buf.WriteString(`<D:`)
			buf.WriteString(prop)
			buf.WriteString(`/>`)
		}
	}
	buf.WriteString(`</D:prop>`)

	buf.WriteString(`</D:sync-collection>`)

	return buf.String()
}

func parseSyncResponse(body []byte) (*SyncResponse, error) {
	var multistatus syncMultiStatus
	if err := xml.Unmarshal(body, &multistatus); err != nil {
		return nil, wrapErrorWithType("parseSyncResponse", ErrorTypeInvalidXML, err)
	}

	resp := &SyncResponse{
		SyncToken: multistatus.SyncToken,
		Changes:   make([]SyncChange, 0),
	}

	for _, response := range multistatus.Responses {
		change := SyncChange{
			Href:       response.Href,
			Properties: make(map[string]string),
		}

		for _, propstat := range response.Propstats {
			status := parseStatusCode(propstat.Status)

			if status == 404 {
				change.Deleted = true
				change.Status = status
				continue
			}

			change.Status = status

			if propstat.Prop.GetETag != "" {
				change.ETag = propstat.Prop.GetETag
				change.Properties["getetag"] = propstat.Prop.GetETag
			}

			if propstat.Prop.CalendarData != "" {
				change.CalendarData = propstat.Prop.CalendarData
				change.Properties["calendar-data"] = propstat.Prop.CalendarData
			}

			if propstat.Prop.DisplayName != "" {
				change.Properties["displayname"] = propstat.Prop.DisplayName
			}

			if propstat.Prop.GetContentType != "" {
				change.Properties["getcontenttype"] = propstat.Prop.GetContentType
			}
		}

		resp.Changes = append(resp.Changes, change)
	}

	resp.TotalChanges = len(resp.Changes)

	return resp, nil
}

func (resp *SyncResponse) GetNewItems() []SyncChange {
	var items []SyncChange
	for _, change := range resp.Changes {
		if change.ChangeType() == SyncChangeTypeNew {
			items = append(items, change)
		}
	}
	return items
}

func (resp *SyncResponse) GetModifiedItems() []SyncChange {
	var items []SyncChange
	for _, change := range resp.Changes {
		if change.ChangeType() == SyncChangeTypeModified {
			items = append(items, change)
		}
	}
	return items
}

func (resp *SyncResponse) GetDeletedItems() []SyncChange {
	var items []SyncChange
	for _, change := range resp.Changes {
		if change.ChangeType() == SyncChangeTypeDeleted {
			items = append(items, change)
		}
	}
	return items
}

func (resp *SyncResponse) HasChanges() bool {
	return len(resp.Changes) > 0
}

func intToString(n int) string {
	if n <= 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{'0' + byte(n%10)}, result...)
		n /= 10
	}

	return string(result)
}
func (c *CalDAVClient) SetCacheMaxAge(maxAge time.Duration) {
	c.etagCache.mu.Lock()
	defer c.etagCache.mu.Unlock()
	c.etagCache.maxAge = maxAge
}

func (c *CalDAVClient) SetBatchSize(size int) {
	if size > 0 {
		c.batchSize = size
	}
}

func (c *CalDAVClient) SetPreferDefaults(prefer *PreferHeader) {
	c.preferDefaults = prefer
}

func (c *CalDAVClient) GetWithETag(ctx context.Context, path string) (*ETagEntry, error) {
	c.etagCache.mu.RLock()
	entry, exists := c.etagCache.entries[path]
	c.etagCache.mu.RUnlock()

	if exists && time.Since(entry.CachedAt) < c.etagCache.maxAge {
		req, err := c.prepareRequest(ctx, "HEAD", path, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("If-None-Match", entry.ETag)

		resp, err := c.GetHTTPClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}()

		if resp.StatusCode == http.StatusNotModified {
			return entry, nil
		}
	}

	req, err := c.prepareRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	c.applyPreferHeader(req, c.preferDefaults)

	resp, err := c.GetHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		h := sha256.Sum256(body)
		etag = hex.EncodeToString(h[:])
	}

	lastMod, _ := http.ParseTime(resp.Header.Get("Last-Modified"))

	newEntry := &ETagEntry{
		ETag:         etag,
		Content:      body,
		LastModified: lastMod,
		CachedAt:     time.Now(),
		Size:         int64(len(body)),
	}

	c.etagCache.mu.Lock()
	c.etagCache.entries[path] = newEntry
	c.etagCache.mu.Unlock()

	return newEntry, nil
}

func (c *CalDAVClient) InvalidateCache(path string) {
	c.etagCache.mu.Lock()
	defer c.etagCache.mu.Unlock()
	delete(c.etagCache.entries, path)
}

func (c *CalDAVClient) ClearCache() {
	c.etagCache.mu.Lock()
	defer c.etagCache.mu.Unlock()
	c.etagCache.entries = make(map[string]*ETagEntry)
}

func (c *CalDAVClient) applyPreferHeader(req *http.Request, prefer *PreferHeader) {
	if prefer == nil {
		return
	}

	var parts []string

	if prefer.ReturnMinimal {
		parts = append(parts, "return=minimal")
	} else if prefer.ReturnRepresentation {
		parts = append(parts, "return=representation")
	}

	if prefer.Wait != nil {
		parts = append(parts, fmt.Sprintf("wait=%d", int(prefer.Wait.Seconds())))
	}

	if prefer.HandlingStrict {
		parts = append(parts, "handling=strict")
	} else if prefer.HandlingLenient {
		parts = append(parts, "handling=lenient")
	}

	if prefer.RespondAsync {
		parts = append(parts, "respond-async")
	}

	if prefer.DepthNoroot {
		parts = append(parts, "depth-noroot")
	}

	if prefer.MaxResults != nil {
		parts = append(parts, fmt.Sprintf("max-results=%d", *prefer.MaxResults))
	}

	if len(parts) > 0 {
		req.Header.Set("Prefer", strings.Join(parts, ", "))
	}
}

func (c *CalDAVClient) BatchExecute(ctx context.Context, operations []BatchOperation) ([]BatchResult, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	results := make([]BatchResult, len(operations))
	chunks := c.chunkOperations(operations, c.batchSize)

	for i, chunk := range chunks {
		chunkResults, err := c.executeBatchChunk(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("batch chunk %d failed: %w", i, err)
		}

		baseIdx := i * c.batchSize
		for j, result := range chunkResults {
			results[baseIdx+j] = result
		}
	}

	return results, nil
}

func (c *CalDAVClient) chunkOperations(operations []BatchOperation, size int) [][]BatchOperation {
	var chunks [][]BatchOperation

	for i := 0; i < len(operations); i += size {
		end := i + size
		if end > len(operations) {
			end = len(operations)
		}
		chunks = append(chunks, operations[i:end])
	}

	return chunks
}

func (c *CalDAVClient) executeBatchChunk(ctx context.Context, operations []BatchOperation) ([]BatchResult, error) {
	results := make([]BatchResult, len(operations))
	var wg sync.WaitGroup
	errChan := make(chan error, len(operations))

	for i, op := range operations {
		wg.Add(1)
		go func(idx int, operation BatchOperation) {
			defer wg.Done()

			result, err := c.executeSingleOperation(ctx, operation)
			if err != nil {
				errChan <- err
				return
			}
			results[idx] = result
		}(i, op)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

func (c *CalDAVClient) executeSingleOperation(ctx context.Context, op BatchOperation) (BatchResult, error) {
	var body io.Reader
	if len(op.Body) > 0 {
		body = strings.NewReader(string(op.Body))
	}

	req, err := c.prepareRequest(ctx, op.Method, op.Path, body)
	if err != nil {
		return BatchResult{Error: err}, err
	}

	for key, value := range op.Headers {
		req.Header.Set(key, value)
	}

	if op.ETag != "" {
		req.Header.Set("ETag", op.ETag)
	}
	if op.IfMatch != "" {
		req.Header.Set("If-Match", op.IfMatch)
	}
	if op.IfNoneMatch != "" {
		req.Header.Set("If-None-Match", op.IfNoneMatch)
	}

	resp, err := c.GetHTTPClient().Do(req)
	if err != nil {
		return BatchResult{Error: err}, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return BatchResult{Error: err}, err
	}

	headers := make(map[string]string)
	for key := range resp.Header {
		headers[key] = resp.Header.Get(key)
	}

	return BatchResult{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       respBody,
		ETag:       resp.Header.Get("ETag"),
	}, nil
}

func (c *CalDAVClient) DeltaSync(ctx context.Context, calendarPath string) (*DeltaSyncState, error) {
	c.syncMu.RLock()
	state, exists := c.deltaStates[calendarPath]
	c.syncMu.RUnlock()

	if !exists {
		state = &DeltaSyncState{
			Resources: make(map[string]*DeltaResource),
			LastSync:  time.Time{},
		}
	}

	syncReq := c.buildSyncRequest(state.SyncToken)

	// Use the CalDAVClient's report method which includes XML validation
	resp, err := c.report(ctx, calendarPath, []byte(syncReq))
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	newState, err := c.parseDeltaSyncResponse(body, state)
	if err != nil {
		return nil, err
	}

	newState.LastSync = time.Now()

	c.syncMu.Lock()
	c.deltaStates[calendarPath] = newState
	c.syncMu.Unlock()

	return newState, nil
}

func (c *CalDAVClient) buildSyncRequest(syncToken string) string {
	var syncTokenXML string
	if syncToken != "" {
		syncTokenXML = fmt.Sprintf("<sync-token>%s</sync-token>", syncToken)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<d:sync-collection xmlns:d="DAV:">
  %s
  <sync-level>1</sync-level>
  <prop>
    <getetag/>
    <getcontenttype/>
    <getcontentlength/>
    <getlastmodified/>
  </prop>
</d:sync-collection>`, syncTokenXML)
}

func (c *CalDAVClient) parseDeltaSyncResponse(body []byte, oldState *DeltaSyncState) (*DeltaSyncState, error) {
	newState := initializeDeltaSyncState(oldState)
	bodyStr := string(body)

	responses := strings.Split(bodyStr, "<response>")
	for _, resp := range responses[1:] {
		processDeltaSyncResponseItem(resp, newState)
	}

	extractSyncToken(bodyStr, newState)
	return newState, nil
}

func initializeDeltaSyncState(oldState *DeltaSyncState) *DeltaSyncState {
	newState := &DeltaSyncState{
		Resources:      make(map[string]*DeltaResource),
		PendingDeletes: []string{},
	}

	if oldState != nil && oldState.Resources != nil {
		for href, resource := range oldState.Resources {
			newState.Resources[href] = resource
		}
	}

	return newState
}

func processDeltaSyncResponseItem(resp string, state *DeltaSyncState) {
	href := extractXMLValue(resp, "<href>", "</href>")
	status := extractHTTPStatus(resp)

	if status == 200 {
		handleSuccessfulDeltaItem(resp, href, state)
	} else if status == 404 && href != "" {
		handleDeletedDeltaItem(href, state)
	}
}

func extractXMLValue(content, startTag, endTag string) string {
	if start := strings.Index(content, startTag); start != -1 {
		end := strings.Index(content[start:], endTag)
		if end != -1 {
			return content[start+len(startTag) : start+end]
		}
	}
	return ""
}

func extractHTTPStatus(resp string) int {
	if statusStart := strings.Index(resp, "HTTP/1.1 "); statusStart != -1 {
		statusEnd := statusStart + 9
		if statusEnd+3 <= len(resp) {
			statusCode := resp[statusEnd : statusEnd+3]
			status, _ := strconv.Atoi(statusCode)
			return status
		}
	}
	return 0
}

func handleSuccessfulDeltaItem(resp, href string, state *DeltaSyncState) {
	if href == "" {
		return
	}

	etag := extractXMLValue(resp, "<getetag>", "</getetag>")
	state.Resources[href] = &DeltaResource{
		Href: href,
		ETag: etag,
	}
}

func handleDeletedDeltaItem(href string, state *DeltaSyncState) {
	delete(state.Resources, href)
	state.PendingDeletes = append(state.PendingDeletes, href)
}

func extractSyncToken(bodyStr string, state *DeltaSyncState) {
	state.SyncToken = extractXMLValue(bodyStr, "<sync-token>", "</sync-token>")
}

func (c *CalDAVClient) GetDeltaResources(calendarPath string) (map[string]*DeltaResource, error) {
	c.syncMu.RLock()
	defer c.syncMu.RUnlock()

	state, exists := c.deltaStates[calendarPath]
	if !exists {
		return nil, fmt.Errorf("no delta sync state for %s", calendarPath)
	}

	return state.Resources, nil
}

func (c *CalDAVClient) CompactCache() {
	c.etagCache.mu.Lock()
	defer c.etagCache.mu.Unlock()

	now := time.Now()
	for path, entry := range c.etagCache.entries {
		if now.Sub(entry.CachedAt) > c.etagCache.maxAge {
			delete(c.etagCache.entries, path)
		}
	}
}

func (c *CalDAVClient) GetCacheStats() (entries int, totalSize int64) {
	c.etagCache.mu.RLock()
	defer c.etagCache.mu.RUnlock()

	entries = len(c.etagCache.entries)
	for _, entry := range c.etagCache.entries {
		totalSize += entry.Size
	}

	return entries, totalSize
}

func (c *CalDAVClient) PreloadCache(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	operations := make([]BatchOperation, len(paths))
	for i, path := range paths {
		operations[i] = BatchOperation{
			Method: "GET",
			Path:   path,
			Headers: map[string]string{
				"Accept": "text/calendar",
			},
		}
	}

	results, err := c.BatchExecute(ctx, operations)
	if err != nil {
		return err
	}

	c.etagCache.mu.Lock()
	defer c.etagCache.mu.Unlock()

	for i, result := range results {
		if result.StatusCode == http.StatusOK {
			etag := result.ETag
			if etag == "" {
				h := sha256.Sum256(result.Body)
				etag = hex.EncodeToString(h[:])
			}

			c.etagCache.entries[paths[i]] = &ETagEntry{
				ETag:     etag,
				Content:  result.Body,
				CachedAt: time.Now(),
				Size:     int64(len(result.Body)),
			}
		}
	}

	return nil
}
