package caldav

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// validateEventForCreation validates that an event has the minimum required properties for creation.
func validateEventForCreation(event *CalendarObject) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	if event.Summary == "" {
		return fmt.Errorf("event summary is required")
	}

	if event.StartTime == nil {
		return fmt.Errorf("event start time is required")
	}

	return nil
}

// validateEventForUpdate validates that an event has the required properties for update.
func validateEventForUpdate(event *CalendarObject) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	if event.UID == "" {
		return fmt.Errorf("event UID is required for update")
	}

	if event.Summary == "" {
		return fmt.Errorf("event summary is required")
	}

	if event.StartTime == nil {
		return fmt.Errorf("event start time is required")
	}

	return nil
}

// generateUID generates a unique identifier for a calendar event.
func generateUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("%d@go-icloud-caldav", timestamp)
	}
	return fmt.Sprintf("%s@go-icloud-caldav", hex.EncodeToString(b))
}

// buildEventURL constructs the full URL for an event.
func buildEventURL(baseURL, calendarPath, uid string) string {
	if !strings.HasPrefix(calendarPath, "/") {
		calendarPath = "/" + calendarPath
	}
	if !strings.HasSuffix(calendarPath, "/") {
		calendarPath += "/"
	}
	return fmt.Sprintf("%s%s%s.ics", baseURL, calendarPath, uid)
}

// buildEventPath constructs the path for an event within a calendar.
func buildEventPath(calendarPath, uid string) string {
	if !strings.HasPrefix(calendarPath, "/") {
		calendarPath = "/" + calendarPath
	}
	if !strings.HasSuffix(calendarPath, "/") {
		calendarPath += "/"
	}
	return fmt.Sprintf("%s%s.ics", calendarPath, uid)
}

// generateICalendar generates iCalendar format data from a CalendarObject.
func generateICalendar(event *CalendarObject) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event cannot be nil")
	}

	var builder strings.Builder
	builder.WriteString("BEGIN:VCALENDAR\r\n")
	builder.WriteString("VERSION:2.0\r\n")
	builder.WriteString("PRODID:-//go-icloud-caldav//EN\r\n")
	builder.WriteString("CALSCALE:GREGORIAN\r\n")
	builder.WriteString("BEGIN:VEVENT\r\n")

	if event.UID != "" {
		builder.WriteString(fmt.Sprintf("UID:%s\r\n", event.UID))
	}

	now := time.Now().UTC()
	builder.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", formatICalTime(now)))

	if event.Created != nil {
		builder.WriteString(fmt.Sprintf("CREATED:%s\r\n", formatICalTime(*event.Created)))
	} else {
		builder.WriteString(fmt.Sprintf("CREATED:%s\r\n", formatICalTime(now)))
	}

	if event.LastModified != nil {
		builder.WriteString(fmt.Sprintf("LAST-MODIFIED:%s\r\n", formatICalTime(*event.LastModified)))
	} else {
		builder.WriteString(fmt.Sprintf("LAST-MODIFIED:%s\r\n", formatICalTime(now)))
	}

	if event.StartTime != nil {
		builder.WriteString(fmt.Sprintf("DTSTART:%s\r\n", formatICalTime(*event.StartTime)))
	}

	if event.EndTime != nil {
		builder.WriteString(fmt.Sprintf("DTEND:%s\r\n", formatICalTime(*event.EndTime)))
	} else if event.StartTime != nil {
		endTime := event.StartTime.Add(time.Hour)
		builder.WriteString(fmt.Sprintf("DTEND:%s\r\n", formatICalTime(endTime)))
	}

	if event.Summary != "" {
		builder.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICalText(event.Summary)))
	}

	if event.Description != "" {
		builder.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICalText(event.Description)))
	}

	if event.Location != "" {
		builder.WriteString(fmt.Sprintf("LOCATION:%s\r\n", escapeICalText(event.Location)))
	}

	if event.Status != "" {
		builder.WriteString(fmt.Sprintf("STATUS:%s\r\n", event.Status))
	} else {
		builder.WriteString("STATUS:CONFIRMED\r\n")
	}

	if event.Organizer != "" {
		builder.WriteString(fmt.Sprintf("ORGANIZER:%s\r\n", event.Organizer))
	}

	for _, attendee := range event.Attendees {
		builder.WriteString(fmt.Sprintf("ATTENDEE:%s\r\n", attendee))
	}

	builder.WriteString("END:VEVENT\r\n")
	builder.WriteString("END:VCALENDAR\r\n")

	return builder.String(), nil
}

// formatICalTime formats a time.Time value in iCalendar format (YYYYMMDDTHHMMSSZ).
func formatICalTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// escapeICalText escapes special characters in iCalendar text values.
func escapeICalText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, ",", "\\,")
	return text
}
