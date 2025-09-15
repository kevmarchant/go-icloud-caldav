package caldav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AlarmAction string

const (
	AlarmActionDisplay AlarmAction = "DISPLAY"
	AlarmActionEmail   AlarmAction = "EMAIL"
	AlarmActionAudio   AlarmAction = "AUDIO"
)

type AlarmTriggerType string

const (
	AlarmTriggerRelative AlarmTriggerType = "RELATIVE"
	AlarmTriggerAbsolute AlarmTriggerType = "ABSOLUTE"
)

type AlarmConfig struct {
	Action      AlarmAction
	Trigger     string
	Description string
	Summary     string
	Duration    string
	Repeat      int
	Attendees   []string
	Attach      string
}

func (c *CalDAVClient) AddAlarmToEvent(ctx context.Context, eventPath string, alarm *AlarmConfig) error {
	eventPath = normalizeEventPath(eventPath, c.baseURL)

	event, etag, err := c.fetchEventForAlarmOperation(ctx, eventPath)
	if err != nil {
		return err
	}

	alarmComponent, err := prepareAlarmComponent(alarm)
	if err != nil {
		return err
	}

	modifiedICal := insertAlarmIntoEvent(event.CalendarData, alarmComponent)

	return c.updateEventWithAlarm(ctx, event, modifiedICal, etag)
}

func (c *CalDAVClient) UpdateAlarm(ctx context.Context, eventPath string, alarmIndex int, alarm *AlarmConfig) error {
	eventPath = normalizeEventPath(eventPath, c.baseURL)

	event, etag, err := c.fetchEventForAlarmOperation(ctx, eventPath)
	if err != nil {
		return err
	}

	modifiedICal, err := c.validateAndUpdateAlarm(event, alarmIndex, alarm)
	if err != nil {
		return err
	}

	return c.updateEventWithAlarm(ctx, event, modifiedICal, etag)
}

func (c *CalDAVClient) RemoveAlarm(ctx context.Context, eventPath string, alarmIndex int) error {
	if !strings.HasPrefix(eventPath, "http://") && !strings.HasPrefix(eventPath, "https://") {
		if !strings.HasPrefix(eventPath, "/") {
			eventPath = "/" + eventPath
		}
		eventPath = c.baseURL + eventPath
	}

	event, etag, err := c.GetEventByPath(ctx, eventPath)
	if err != nil {
		return fmt.Errorf("fetching event: %w", err)
	}

	parsedCal, err := ParseICalendar(event.CalendarData)
	if err != nil {
		return fmt.Errorf("parsing calendar data: %w", err)
	}

	if len(parsedCal.Events) == 0 {
		return fmt.Errorf("no events found in calendar data")
	}

	if alarmIndex < 0 || alarmIndex >= len(parsedCal.Events[0].Alarms) {
		return fmt.Errorf("alarm index %d out of range", alarmIndex)
	}

	modifiedICal := removeAlarmFromEvent(event.CalendarData, alarmIndex)

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

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	case http.StatusNotFound:
		return &EventNotFoundError{UID: ""}
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (c *CalDAVClient) RemoveAllAlarms(ctx context.Context, eventPath string) error {
	if !strings.HasPrefix(eventPath, "http://") && !strings.HasPrefix(eventPath, "https://") {
		if !strings.HasPrefix(eventPath, "/") {
			eventPath = "/" + eventPath
		}
		eventPath = c.baseURL + eventPath
	}

	event, etag, err := c.GetEventByPath(ctx, eventPath)
	if err != nil {
		return fmt.Errorf("fetching event: %w", err)
	}

	modifiedICal := removeAllAlarmsFromEvent(event.CalendarData)

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

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusPreconditionFailed:
		return &ETagMismatchError{Expected: etag}
	case http.StatusNotFound:
		return &EventNotFoundError{UID: ""}
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

func (c *CalDAVClient) GetEventByPath(ctx context.Context, eventPath string) (*CalendarObject, string, error) {
	if !strings.HasPrefix(eventPath, "http://") && !strings.HasPrefix(eventPath, "https://") {
		eventPath = c.baseURL + eventPath
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventPath, nil)
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

	etag := resp.Header.Get("ETag")

	return &CalendarObject{
		CalendarData: string(body),
		ETag:         etag,
		Href:         eventPath,
	}, etag, nil
}

func validateAlarm(alarm *AlarmConfig) error {
	if alarm == nil {
		return fmt.Errorf("alarm configuration cannot be nil")
	}

	if alarm.Action == "" {
		return fmt.Errorf("alarm action is required")
	}

	validActions := map[AlarmAction]bool{
		AlarmActionDisplay: true,
		AlarmActionEmail:   true,
		AlarmActionAudio:   true,
	}

	if !validActions[alarm.Action] {
		return fmt.Errorf("invalid alarm action: %s", alarm.Action)
	}

	if alarm.Trigger == "" {
		return fmt.Errorf("alarm trigger is required")
	}

	if alarm.Action == AlarmActionDisplay && alarm.Description == "" {
		return fmt.Errorf("description is required for DISPLAY alarms")
	}

	if alarm.Action == AlarmActionEmail {
		if alarm.Description == "" {
			return fmt.Errorf("description is required for EMAIL alarms")
		}
		if alarm.Summary == "" {
			return fmt.Errorf("summary is required for EMAIL alarms")
		}
		if len(alarm.Attendees) == 0 {
			return fmt.Errorf("at least one attendee is required for EMAIL alarms")
		}
	}

	if alarm.Repeat > 0 && alarm.Duration == "" {
		return fmt.Errorf("duration is required when repeat is specified")
	}

	return nil
}

func generateAlarmComponent(alarm *AlarmConfig) string {
	var builder strings.Builder

	builder.WriteString("BEGIN:VALARM\r\n")
	builder.WriteString(fmt.Sprintf("ACTION:%s\r\n", alarm.Action))
	builder.WriteString(fmt.Sprintf("TRIGGER:%s\r\n", alarm.Trigger))

	if alarm.Description != "" {
		builder.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICalText(alarm.Description)))
	}

	if alarm.Summary != "" {
		builder.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICalText(alarm.Summary)))
	}

	if alarm.Duration != "" {
		builder.WriteString(fmt.Sprintf("DURATION:%s\r\n", alarm.Duration))
	}

	if alarm.Repeat > 0 {
		builder.WriteString(fmt.Sprintf("REPEAT:%d\r\n", alarm.Repeat))
	}

	for _, attendee := range alarm.Attendees {
		builder.WriteString(fmt.Sprintf("ATTENDEE:%s\r\n", attendee))
	}

	if alarm.Attach != "" {
		builder.WriteString(fmt.Sprintf("ATTACH:%s\r\n", alarm.Attach))
	}

	builder.WriteString("END:VALARM\r\n")

	return builder.String()
}

func insertAlarmIntoEvent(icalData string, alarmComponent string) string {
	icalData = strings.ReplaceAll(icalData, "\r\n", "\n")
	icalData = strings.ReplaceAll(icalData, "\r", "\n")
	lines := strings.Split(icalData, "\n")

	alarmComponent = strings.ReplaceAll(alarmComponent, "\r\n", "\n")
	alarmComponent = strings.TrimSuffix(alarmComponent, "\n")
	alarmLines := strings.Split(alarmComponent, "\n")

	var result []string
	for _, line := range lines {
		if strings.HasPrefix(line, "END:VEVENT") {
			result = append(result, alarmLines...)
		}
		result = append(result, line)
	}

	return strings.Join(result, "\r\n")
}

func replaceAlarmInEvent(icalData string, alarmIndex int, newAlarmComponent string) string {
	icalData = strings.ReplaceAll(icalData, "\r\n", "\n")
	icalData = strings.ReplaceAll(icalData, "\r", "\n")
	lines := strings.Split(icalData, "\n")

	newAlarmComponent = strings.ReplaceAll(newAlarmComponent, "\r\n", "\n")
	newAlarmComponent = strings.TrimSuffix(newAlarmComponent, "\n")
	var result []string
	alarmCount := 0
	skipAlarm := false

	for _, line := range lines {
		if strings.HasPrefix(line, "BEGIN:VALARM") {
			if alarmCount == alarmIndex {
				skipAlarm = true
				alarmLines := strings.Split(newAlarmComponent, "\n")
				result = append(result, alarmLines...)
			} else {
				result = append(result, line)
			}
			alarmCount++
		} else if strings.HasPrefix(line, "END:VALARM") {
			if !skipAlarm {
				result = append(result, line)
			}
			skipAlarm = false
		} else if !skipAlarm {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\r\n")
}

func removeAlarmFromEvent(icalData string, alarmIndex int) string {
	icalData = strings.ReplaceAll(icalData, "\r\n", "\n")
	icalData = strings.ReplaceAll(icalData, "\r", "\n")
	lines := strings.Split(icalData, "\n")
	var result []string
	alarmCount := 0
	skipAlarm := false

	for _, line := range lines {
		if strings.HasPrefix(line, "BEGIN:VALARM") {
			if alarmCount == alarmIndex {
				skipAlarm = true
			} else {
				result = append(result, line)
			}
			alarmCount++
		} else if strings.HasPrefix(line, "END:VALARM") {
			if !skipAlarm {
				result = append(result, line)
			}
			skipAlarm = false
		} else if !skipAlarm {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\r\n")
}

func removeAllAlarmsFromEvent(icalData string) string {
	icalData = strings.ReplaceAll(icalData, "\r\n", "\n")
	icalData = strings.ReplaceAll(icalData, "\r", "\n")
	lines := strings.Split(icalData, "\n")
	var result []string
	inAlarm := false

	for _, line := range lines {
		if strings.HasPrefix(line, "BEGIN:VALARM") {
			inAlarm = true
		} else if strings.HasPrefix(line, "END:VALARM") {
			inAlarm = false
		} else if !inAlarm {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\r\n")
}

func ParseAlarmTrigger(trigger string) (time.Duration, bool, error) {
	trigger = strings.TrimSpace(trigger)

	if strings.HasPrefix(trigger, "-P") || strings.HasPrefix(trigger, "P") {
		return parseISO8601Duration(trigger)
	}

	if strings.Contains(trigger, "T") && len(trigger) == 16 {
		_, err := time.Parse("20060102T150405Z", trigger)
		if err == nil {
			return 0, false, nil
		}
	}

	return 0, false, fmt.Errorf("invalid trigger format: %s", trigger)
}

func parseISO8601Duration(duration string) (time.Duration, bool, error) {
	return parseISO8601DurationSimplified(duration)
}

func CreateRelativeAlarmTrigger(duration time.Duration, before bool) string {
	prefix := ""
	if before {
		prefix = "-"
	}

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	var result string
	result = prefix + "P"

	if days > 0 {
		result += fmt.Sprintf("%dD", days)
	}

	if hours > 0 || minutes > 0 || seconds > 0 {
		result += "T"
		if hours > 0 {
			result += fmt.Sprintf("%dH", hours)
		}
		if minutes > 0 {
			result += fmt.Sprintf("%dM", minutes)
		}
		if seconds > 0 {
			result += fmt.Sprintf("%dS", seconds)
		}
	}

	if result == prefix+"P" {
		result = prefix + "PT0S"
	}

	return result
}

func CreateAbsoluteAlarmTrigger(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
