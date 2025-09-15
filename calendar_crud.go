package caldav

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CreateCalendar creates a new calendar collection at the specified path.
// The calendar must have at least a DisplayName set.
func (c *CalDAVClient) CreateCalendar(homeSetPath string, calendar *Calendar) error {
	return c.CreateCalendarWithContext(context.Background(), homeSetPath, calendar)
}

// CreateCalendarWithContext creates a new calendar with the provided context.
func (c *CalDAVClient) CreateCalendarWithContext(ctx context.Context, homeSetPath string, calendar *Calendar) error {
	if err := validateCalendarForCreation(calendar); err != nil {
		return fmt.Errorf("calendar validation failed: %w", err)
	}

	if calendar.Name == "" {
		calendar.Name = sanitizeCalendarName(calendar.DisplayName)
	}

	calendarPath := buildCalendarURL(c.baseURL, homeSetPath, calendar.Name)

	xmlBody := buildMakeCalendarXML(calendar)

	req, err := http.NewRequestWithContext(ctx, "MKCALENDAR", calendarPath, bytes.NewBufferString(xmlBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", userAgent)

	if c.debugHTTP {
		c.logger.Debug("Creating calendar", "url", calendarPath, "name", calendar.DisplayName)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusCreated:
		calendar.Href = calendarPath
		return nil
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusConflict:
		return ErrCalendarAlreadyExists
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// UpdateCalendar updates an existing calendar's properties.
// Can update DisplayName, Description, Color, and CalendarTimeZone.
func (c *CalDAVClient) UpdateCalendar(calendarPath string, updates *CalendarPropertyUpdate) error {
	return c.UpdateCalendarWithContext(context.Background(), calendarPath, updates)
}

// UpdateCalendarWithContext updates a calendar with the provided context.
func (c *CalDAVClient) UpdateCalendarWithContext(ctx context.Context, calendarPath string, updates *CalendarPropertyUpdate) error {
	if updates == nil || !updates.hasUpdates() {
		return fmt.Errorf("no updates provided")
	}

	xmlBody := buildUpdateCalendarXML(updates)

	calendarURL := normalizeCalendarPath(c.baseURL, calendarPath)

	req, err := http.NewRequestWithContext(ctx, "PROPPATCH", calendarURL, bytes.NewBufferString(xmlBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", userAgent)

	if c.debugHTTP {
		c.logger.Debug("Updating calendar", "url", calendarURL)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusMultiStatus:
		return nil
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrCalendarNotFound
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// DeleteCalendar deletes a calendar collection.
func (c *CalDAVClient) DeleteCalendar(calendarPath string) error {
	return c.DeleteCalendarWithContext(context.Background(), calendarPath)
}

// DeleteCalendarWithContext deletes a calendar with the provided context.
func (c *CalDAVClient) DeleteCalendarWithContext(ctx context.Context, calendarPath string) error {
	calendarURL := normalizeCalendarPath(c.baseURL, calendarPath)

	req, err := http.NewRequestWithContext(ctx, "DELETE", calendarURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", userAgent)

	if c.debugHTTP {
		c.logger.Debug("Deleting calendar", "url", calendarURL)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return nil
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// CalendarPropertyUpdate contains properties that can be updated on a calendar.
type CalendarPropertyUpdate struct {
	DisplayName      *string
	Description      *string
	Color            *string
	CalendarTimeZone *string
}

func (u *CalendarPropertyUpdate) hasUpdates() bool {
	return u.DisplayName != nil || u.Description != nil ||
		u.Color != nil || u.CalendarTimeZone != nil
}

func validateCalendarForCreation(calendar *Calendar) error {
	if calendar == nil {
		return fmt.Errorf("calendar cannot be nil")
	}

	if calendar.DisplayName == "" {
		return fmt.Errorf("calendar must have a display name")
	}
	return nil
}

func sanitizeCalendarName(displayName string) string {
	name := strings.ToLower(displayName)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "*", "-")
	name = strings.ReplaceAll(name, "?", "-")
	name = strings.ReplaceAll(name, "\"", "-")
	name = strings.ReplaceAll(name, "<", "-")
	name = strings.ReplaceAll(name, ">", "-")
	name = strings.ReplaceAll(name, "|", "-")
	return name
}

func buildCalendarURL(baseURL, homeSetPath, calendarName string) string {
	if !strings.HasSuffix(homeSetPath, "/") {
		homeSetPath += "/"
	}
	if !strings.HasSuffix(calendarName, "/") {
		calendarName += "/"
	}
	return fmt.Sprintf("%s%s%s", baseURL, homeSetPath, calendarName)
}

func normalizeCalendarPath(baseURL, calendarPath string) string {
	if strings.HasPrefix(calendarPath, "http://") || strings.HasPrefix(calendarPath, "https://") {
		return calendarPath
	}
	if !strings.HasPrefix(calendarPath, "/") {
		calendarPath = "/" + calendarPath
	}
	return baseURL + calendarPath
}

func buildMakeCalendarXML(calendar *Calendar) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:A="http://apple.com/ns/ical/">`)
	sb.WriteString(`<D:set>`)
	sb.WriteString(`<D:prop>`)

	sb.WriteString(`<D:displayname>`)
	sb.WriteString(escapeXML(calendar.DisplayName))
	sb.WriteString(`</D:displayname>`)

	if calendar.Description != "" {
		sb.WriteString(`<C:calendar-description>`)
		sb.WriteString(escapeXML(calendar.Description))
		sb.WriteString(`</C:calendar-description>`)
	}

	if calendar.Color != "" {
		sb.WriteString(`<A:calendar-color>`)
		sb.WriteString(escapeXML(calendar.Color))
		sb.WriteString(`</A:calendar-color>`)
	}

	if calendar.CalendarTimeZone != "" {
		sb.WriteString(`<C:calendar-timezone>`)
		sb.WriteString(escapeXML(calendar.CalendarTimeZone))
		sb.WriteString(`</C:calendar-timezone>`)
	}

	if len(calendar.SupportedComponents) > 0 {
		sb.WriteString(`<C:supported-calendar-component-set>`)
		for _, comp := range calendar.SupportedComponents {
			sb.WriteString(`<C:comp name="`)
			sb.WriteString(escapeXML(comp))
			sb.WriteString(`"/>`)
		}
		sb.WriteString(`</C:supported-calendar-component-set>`)
	} else {
		sb.WriteString(`<C:supported-calendar-component-set>`)
		sb.WriteString(`<C:comp name="VEVENT"/>`)
		sb.WriteString(`</C:supported-calendar-component-set>`)
	}

	sb.WriteString(`</D:prop>`)
	sb.WriteString(`</D:set>`)
	sb.WriteString(`</C:mkcalendar>`)

	return sb.String()
}

func buildUpdateCalendarXML(updates *CalendarPropertyUpdate) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:A="http://apple.com/ns/ical/">`)
	sb.WriteString(`<D:set>`)
	sb.WriteString(`<D:prop>`)

	if updates.DisplayName != nil {
		sb.WriteString(`<D:displayname>`)
		sb.WriteString(escapeXML(*updates.DisplayName))
		sb.WriteString(`</D:displayname>`)
	}

	if updates.Description != nil {
		sb.WriteString(`<C:calendar-description>`)
		sb.WriteString(escapeXML(*updates.Description))
		sb.WriteString(`</C:calendar-description>`)
	}

	if updates.Color != nil {
		sb.WriteString(`<A:calendar-color>`)
		sb.WriteString(escapeXML(*updates.Color))
		sb.WriteString(`</A:calendar-color>`)
	}

	if updates.CalendarTimeZone != nil {
		sb.WriteString(`<C:calendar-timezone>`)
		sb.WriteString(escapeXML(*updates.CalendarTimeZone))
		sb.WriteString(`</C:calendar-timezone>`)
	}

	sb.WriteString(`</D:prop>`)
	sb.WriteString(`</D:set>`)
	sb.WriteString(`</D:propertyupdate>`)

	return sb.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
