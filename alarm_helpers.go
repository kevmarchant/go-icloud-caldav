package caldav

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (c *CalDAVClient) fetchEventForAlarmOperation(ctx context.Context, eventPath string) (*CalendarObject, string, error) {
	event, etag, err := c.GetEventByPath(ctx, eventPath)
	if err != nil {
		return nil, "", fmt.Errorf("fetching event: %w", err)
	}

	if event.CalendarData == "" {
		return nil, "", fmt.Errorf("event has no calendar data")
	}

	return event, etag, nil
}

func validateAlarmIndex(parsedCal *ParsedCalendarData, alarmIndex int) error {
	if len(parsedCal.Events) == 0 {
		return fmt.Errorf("no events found in calendar data")
	}

	if alarmIndex < 0 || alarmIndex >= len(parsedCal.Events[0].Alarms) {
		return fmt.Errorf("alarm index %d out of range", alarmIndex)
	}

	return nil
}

func (c *CalDAVClient) updateEventWithAlarm(ctx context.Context, event *CalendarObject, modifiedICal, etag string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, event.Href, strings.NewReader(modifiedICal))
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

	return handleAlarmUpdateResponse(resp, etag)
}

func handleAlarmUpdateResponse(resp *http.Response, etag string) error {
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	case http.StatusNotFound:
		return &EventNotFoundError{UID: ""}
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		body, _ := readResponseBody(resp)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func prepareAlarmComponent(alarm *AlarmConfig) (string, error) {
	if alarm == nil {
		return "", fmt.Errorf("alarm configuration cannot be nil")
	}

	if err := validateAlarm(alarm); err != nil {
		return "", fmt.Errorf("invalid alarm configuration: %w", err)
	}

	return generateAlarmComponent(alarm), nil
}

func (c *CalDAVClient) validateAndUpdateAlarm(event *CalendarObject, alarmIndex int, alarm *AlarmConfig) (string, error) {
	alarmComponent, err := prepareAlarmComponent(alarm)
	if err != nil {
		return "", err
	}

	parsedCal, err := ParseICalendar(event.CalendarData)
	if err != nil {
		return "", fmt.Errorf("parsing calendar data: %w", err)
	}

	if err := validateAlarmIndex(parsedCal, alarmIndex); err != nil {
		return "", err
	}

	return replaceAlarmInEvent(event.CalendarData, alarmIndex, alarmComponent), nil
}
