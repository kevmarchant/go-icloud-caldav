package caldav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CreateEvent creates a new event in the specified calendar.
// The event must have at least Summary and DTStart properties set.
// Returns an error if the event already exists (based on UID) or if creation fails.
func (c *CalDAVClient) CreateEvent(calendarPath string, event *CalendarObject) error {
	return c.CreateEventWithContext(context.Background(), calendarPath, event)
}

// CreateEventWithContext creates a new event with the provided context.
func (c *CalDAVClient) CreateEventWithContext(ctx context.Context, calendarPath string, event *CalendarObject) error {
	if err := validateEventForCreation(event); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	if event.UID == "" {
		event.UID = generateUID()
	}

	icalData, err := generateICalendar(event)
	if err != nil {
		return fmt.Errorf("generating iCalendar data: %w", err)
	}

	eventURL := buildEventURL(c.baseURL, calendarPath, event.UID)

	req, err := http.NewRequestWithContext(ctx, "PUT", eventURL, bytes.NewBufferString(icalData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("User-Agent", userAgent)

	if c.debugHTTP {
		c.logger.Debug("Creating event", "url", eventURL, "uid", event.UID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusNoContent:
		if etag := resp.Header.Get("ETag"); etag != "" {
			event.ETag = etag
		}
		if c.debugHTTP {
			c.logger.Debug("Event created successfully", "status", resp.StatusCode, "etag", event.ETag)
		}
		return nil
	case http.StatusPreconditionFailed:
		return &EventExistsError{UID: event.UID}
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// UpdateEvent updates an existing event in the specified calendar.
// The etag parameter should be the ETag from a previous fetch to ensure optimistic locking.
// Pass an empty string for etag to force update without checking.
func (c *CalDAVClient) UpdateEvent(calendarPath string, event *CalendarObject, etag string) error {
	return c.UpdateEventWithContext(context.Background(), calendarPath, event, etag)
}

// UpdateEventWithContext updates an existing event with the provided context.
func (c *CalDAVClient) UpdateEventWithContext(ctx context.Context, calendarPath string, event *CalendarObject, etag string) error {
	if event.UID == "" {
		return fmt.Errorf("event UID is required for update")
	}

	if err := validateEventForUpdate(event); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	now := time.Now().UTC()
	event.LastModified = &now

	icalData, err := generateICalendar(event)
	if err != nil {
		return fmt.Errorf("generating iCalendar data: %w", err)
	}

	eventURL := buildEventURL(c.baseURL, calendarPath, event.UID)

	req, err := http.NewRequestWithContext(ctx, "PUT", eventURL, bytes.NewBufferString(icalData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("Authorization", c.authHeader)
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	req.Header.Set("User-Agent", userAgent)

	if c.debugHTTP {
		c.logger.Debug("Updating event", "url", eventURL, "uid", event.UID, "etag", etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		if newEtag := resp.Header.Get("ETag"); newEtag != "" {
			event.ETag = newEtag
		}
		if c.debugHTTP {
			c.logger.Debug("Event updated successfully", "status", resp.StatusCode, "newEtag", event.ETag)
		}
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	case http.StatusNotFound:
		return &EventNotFoundError{UID: event.UID}
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// DeleteEvent deletes an event from the specified calendar.
func (c *CalDAVClient) DeleteEvent(eventPath string) error {
	return c.DeleteEventWithContext(context.Background(), eventPath)
}

// DeleteEventWithContext deletes an event with the provided context.
func (c *CalDAVClient) DeleteEventWithContext(ctx context.Context, eventPath string) error {
	return c.DeleteEventWithETag(ctx, eventPath, "")
}

// DeleteEventWithETag deletes an event only if the ETag matches (for safe deletion).
// Pass an empty string for etag to force deletion without checking.
func (c *CalDAVClient) DeleteEventWithETag(ctx context.Context, eventPath string, etag string) error {
	eventURL := c.baseURL + eventPath
	if !strings.HasSuffix(eventURL, ".ics") {
		eventURL += ".ics"
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", eventURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	req.Header.Set("User-Agent", userAgent)

	if c.debugHTTP {
		c.logger.Debug("Deleting event", "url", eventURL, "etag", etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		if c.debugHTTP {
			c.logger.Debug("Event deleted successfully", "status", resp.StatusCode)
		}
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// DeleteEventByUID deletes an event by its UID from the specified calendar.
func (c *CalDAVClient) DeleteEventByUID(calendarPath string, uid string) error {
	return c.DeleteEventByUIDWithContext(context.Background(), calendarPath, uid)
}

// DeleteEventByUIDWithContext deletes an event by its UID with the provided context.
func (c *CalDAVClient) DeleteEventByUIDWithContext(ctx context.Context, calendarPath string, uid string) error {
	eventPath := buildEventPath(calendarPath, uid)
	return c.DeleteEventWithContext(ctx, eventPath)
}
