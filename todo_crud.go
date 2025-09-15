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

func (c *CalDAVClient) CreateTodo(ctx context.Context, calendarPath string, todo *ParsedTodo) error {
	if err := validateTodo(todo); err != nil {
		return fmt.Errorf("validating todo: %w", err)
	}

	if todo.UID == "" {
		todo.UID = generateUID()
	}

	if todo.DTStamp == nil {
		now := time.Now().UTC()
		todo.DTStamp = &now
	}

	if todo.Created == nil {
		now := time.Now().UTC()
		todo.Created = &now
	}

	if todo.LastModified == nil {
		now := time.Now().UTC()
		todo.LastModified = &now
	}

	if todo.Status == "" {
		todo.Status = "NEEDS-ACTION"
	}

	icalData := generateTodoICalendar(todo)
	todoURL := buildTodoURL(c.baseURL, calendarPath, todo.UID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, todoURL, strings.NewReader(icalData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("If-None-Match", "*")
	req.Header.Set("Authorization", c.authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusPreconditionFailed:
		return &EventExistsError{UID: todo.UID}
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (c *CalDAVClient) UpdateTodo(ctx context.Context, calendarPath string, todo *ParsedTodo, etag string) error {
	if err := validateTodo(todo); err != nil {
		return fmt.Errorf("validating todo: %w", err)
	}

	if todo.UID == "" {
		return fmt.Errorf("UID is required for update")
	}

	now := time.Now().UTC()
	todo.LastModified = &now
	todo.Sequence++

	icalData := generateTodoICalendar(todo)
	todoURL := buildTodoURL(c.baseURL, calendarPath, todo.UID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, todoURL, strings.NewReader(icalData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("Authorization", c.authHeader)
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	case http.StatusNotFound:
		return &EventNotFoundError{UID: todo.UID}
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (c *CalDAVClient) DeleteTodo(ctx context.Context, todoPath string, etag string) error {
	if !strings.HasPrefix(todoPath, "http://") && !strings.HasPrefix(todoPath, "https://") {
		todoPath = c.baseURL + todoPath
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, todoPath, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (c *CalDAVClient) DeleteTodoByUID(ctx context.Context, calendarPath string, uid string) error {
	todoPath := buildTodoURL(c.baseURL, calendarPath, uid)
	return c.DeleteTodo(ctx, todoPath, "")
}

func (c *CalDAVClient) CompleteTodo(ctx context.Context, calendarPath string, uid string, percentComplete int) error {
	todoPath := buildTodoURL(c.baseURL, calendarPath, uid)

	todo, etag, err := c.GetTodo(ctx, todoPath)
	if err != nil {
		return fmt.Errorf("fetching todo: %w", err)
	}

	now := time.Now().UTC()
	todo.PercentComplete = percentComplete

	if percentComplete >= 100 {
		todo.Status = "COMPLETED"
		todo.Completed = &now
		todo.PercentComplete = 100
	} else if percentComplete > 0 {
		todo.Status = "IN-PROCESS"
	} else {
		todo.Status = "NEEDS-ACTION"
	}

	return c.UpdateTodo(ctx, calendarPath, todo, etag)
}

func (c *CalDAVClient) GetTodo(ctx context.Context, todoPath string) (*ParsedTodo, string, error) {
	if !strings.HasPrefix(todoPath, "http://") && !strings.HasPrefix(todoPath, "https://") {
		todoPath = c.baseURL + todoPath
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, todoPath, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, "", &EventNotFoundError{UID: ""}
		}
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading response: %w", err)
	}

	parsedICal, err := ParseICalendar(string(body))
	if err != nil {
		return nil, "", fmt.Errorf("parsing iCalendar: %w", err)
	}

	if len(parsedICal.Todos) == 0 {
		return nil, "", fmt.Errorf("no TODO found in response")
	}

	etag := resp.Header.Get("ETag")
	return &parsedICal.Todos[0], etag, nil
}

func (c *CalDAVClient) GetTodos(ctx context.Context, calendarPath string) ([]ParsedTodo, error) {
	if !strings.HasPrefix(calendarPath, "http://") && !strings.HasPrefix(calendarPath, "https://") {
		if !strings.HasPrefix(calendarPath, "/") {
			calendarPath = "/" + calendarPath
		}
		calendarPath = c.baseURL + calendarPath
	}

	queryXML := `<?xml version="1.0" encoding="UTF-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VTODO"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`

	req, err := http.NewRequestWithContext(ctx, "REPORT", calendarPath, strings.NewReader(queryXML))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "1")
	req.Header.Set("Authorization", c.authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMultiStatus {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	response, err := parseMultiStatusResponse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	var todos []ParsedTodo
	for _, r := range response.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 && ps.Prop.CalendarData != "" {
				parsedICal, err := ParseICalendar(ps.Prop.CalendarData)
				if err != nil {
					continue
				}
				todos = append(todos, parsedICal.Todos...)
			}
		}
	}

	return todos, nil
}

func validateTodo(todo *ParsedTodo) error {
	if todo == nil {
		return fmt.Errorf("todo cannot be nil")
	}

	if todo.Summary == "" {
		return fmt.Errorf("summary is required")
	}

	if todo.Priority < 0 || todo.Priority > 9 {
		return fmt.Errorf("priority must be between 0 and 9")
	}

	if todo.PercentComplete < 0 || todo.PercentComplete > 100 {
		return fmt.Errorf("percent complete must be between 0 and 100")
	}

	validStatuses := map[string]bool{
		"NEEDS-ACTION": true,
		"IN-PROCESS":   true,
		"COMPLETED":    true,
		"CANCELLED":    true,
	}

	if todo.Status != "" && !validStatuses[todo.Status] {
		return fmt.Errorf("invalid status: %s", todo.Status)
	}

	return nil
}

func generateTodoICalendar(todo *ParsedTodo) string {
	var builder strings.Builder

	builder.WriteString("BEGIN:VCALENDAR\r\n")
	builder.WriteString("VERSION:2.0\r\n")
	builder.WriteString("PRODID:-//go-icloud-caldav//EN\r\n")
	builder.WriteString("BEGIN:VTODO\r\n")

	fmt.Fprintf(&builder, "UID:%s\r\n", escapeICalText(todo.UID))

	writeTodoDateProperties(&builder, todo)
	writeTodoMainProperties(&builder, todo)
	writeTodoListProperties(&builder, todo)
	writeTodoRelationships(&builder, todo)
	writeTodoAttachments(&builder, todo)
	writeTodoRequestStatus(&builder, todo)

	builder.WriteString("END:VTODO\r\n")
	builder.WriteString("END:VCALENDAR\r\n")

	return builder.String()
}

func writeTodoDateProperties(builder *strings.Builder, todo *ParsedTodo) {
	if todo.DTStamp != nil {
		fmt.Fprintf(builder, "DTSTAMP:%s\r\n", formatICalTime(*todo.DTStamp))
	}
	if todo.Created != nil {
		fmt.Fprintf(builder, "CREATED:%s\r\n", formatICalTime(*todo.Created))
	}
	if todo.LastModified != nil {
		fmt.Fprintf(builder, "LAST-MODIFIED:%s\r\n", formatICalTime(*todo.LastModified))
	}
	if todo.DTStart != nil {
		fmt.Fprintf(builder, "DTSTART:%s\r\n", formatICalTime(*todo.DTStart))
	}
	if todo.Due != nil {
		fmt.Fprintf(builder, "DUE:%s\r\n", formatICalTime(*todo.Due))
	}
	if todo.Completed != nil {
		fmt.Fprintf(builder, "COMPLETED:%s\r\n", formatICalTime(*todo.Completed))
	}
}

func writeTodoMainProperties(builder *strings.Builder, todo *ParsedTodo) {
	fmt.Fprintf(builder, "SUMMARY:%s\r\n", escapeICalText(todo.Summary))

	if todo.Description != "" {
		fmt.Fprintf(builder, "DESCRIPTION:%s\r\n", escapeICalText(todo.Description))
	}
	if todo.Status != "" {
		fmt.Fprintf(builder, "STATUS:%s\r\n", todo.Status)
	}
	if todo.Priority > 0 {
		fmt.Fprintf(builder, "PRIORITY:%d\r\n", todo.Priority)
	}
	if todo.PercentComplete > 0 {
		fmt.Fprintf(builder, "PERCENT-COMPLETE:%d\r\n", todo.PercentComplete)
	}
	if todo.Sequence > 0 {
		fmt.Fprintf(builder, "SEQUENCE:%d\r\n", todo.Sequence)
	}
	if todo.Class != "" {
		fmt.Fprintf(builder, "CLASS:%s\r\n", todo.Class)
	}
}

func writeTodoListProperties(builder *strings.Builder, todo *ParsedTodo) {
	for _, category := range todo.Categories {
		fmt.Fprintf(builder, "CATEGORIES:%s\r\n", escapeICalText(category))
	}
	for _, contact := range todo.Contacts {
		fmt.Fprintf(builder, "CONTACT:%s\r\n", escapeICalText(contact))
	}
	for _, comment := range todo.Comments {
		fmt.Fprintf(builder, "COMMENT:%s\r\n", escapeICalText(comment))
	}
}

func writeTodoRelationships(builder *strings.Builder, todo *ParsedTodo) {
	for _, related := range todo.RelatedTo {
		if related.RelationType != "" {
			fmt.Fprintf(builder, "RELATED-TO;RELTYPE=%s:%s\r\n", related.RelationType, escapeICalText(related.UID))
		} else {
			fmt.Fprintf(builder, "RELATED-TO:%s\r\n", escapeICalText(related.UID))
		}
	}
}

func writeTodoAttachments(builder *strings.Builder, todo *ParsedTodo) {
	for _, attachment := range todo.Attachments {
		if attachment.URI != "" {
			fmt.Fprintf(builder, "ATTACH:%s\r\n", attachment.URI)
		} else if attachment.Value != "" {
			fmt.Fprintf(builder, "ATTACH;ENCODING=BASE64;VALUE=BINARY:%s\r\n", attachment.Value)
		}
	}
}

func writeTodoRequestStatus(builder *strings.Builder, todo *ParsedTodo) {
	for _, rs := range todo.RequestStatus {
		reqStat := rs.Code
		if rs.Description != "" {
			reqStat += ";" + escapeICalText(rs.Description)
			if rs.ExtraData != "" {
				reqStat += ";" + escapeICalText(rs.ExtraData)
			}
		}
		fmt.Fprintf(builder, "REQUEST-STATUS:%s\r\n", reqStat)
	}
}

func buildTodoURL(baseURL, calendarPath, uid string) string {
	if !strings.HasPrefix(calendarPath, "/") {
		calendarPath = "/" + calendarPath
	}
	if !strings.HasSuffix(calendarPath, "/") {
		calendarPath += "/"
	}
	return fmt.Sprintf("%s%s%s.ics", baseURL, calendarPath, uid)
}
