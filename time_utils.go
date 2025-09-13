package caldav

import (
	"fmt"
	"strings"
	"time"
)

var calDAVTimeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"20060102T150405Z",
	"2006-01-02T15:04:05",
	"20060102T150405",
	"2006-01-02",
	"20060102",
}

// ParseCalDAVTime attempts to parse a time string using common CalDAV formats.
// It returns the parsed time and nil error on success, or zero time and error on failure.
func ParseCalDAVTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, newTypedError("parse.time", ErrorTypeValidation, "empty time value", nil)
	}

	for _, format := range calDAVTimeFormats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, newTypedError("parse.time", ErrorTypeValidation, "unable to parse time: "+value, nil)
}

// ParseCalDAVTimePtr is like ParseCalDAVTime but returns a pointer.
// It returns nil if the time cannot be parsed.
func ParseCalDAVTimePtr(value string) *time.Time {
	if value == "" {
		return nil
	}

	t, err := ParseCalDAVTime(value)
	if err != nil {
		return nil
	}
	return &t
}

// ParseICalPropertyTime parses a time from an iCalendar property line.
// It expects format "PROPERTY:VALUE" and extracts the VALUE part.
func ParseICalPropertyTime(line string) *time.Time {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	return ParseCalDAVTimePtr(parts[1])
}

// ParseCalDAVTimeWithParams parses time considering TZID and other parameters.
func ParseCalDAVTimeWithParams(value string, params map[string]string) *time.Time {
	// For now, just parse the time value
	// In the future, we could handle TZID parameter to parse in specific timezone
	return ParseCalDAVTimePtr(value)
}

// ParseCalDAVTimeDates parses multiple comma-separated date/time values.
func ParseCalDAVTimeDates(value string, params map[string]string) []time.Time {
	var times []time.Time

	// Handle comma-separated values
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if t := ParseCalDAVTimePtr(part); t != nil {
			times = append(times, *t)
		}
	}

	return times
}

// ParseDuration parses an iCalendar duration string.
func ParseDuration(value string) time.Duration {
	// This is a simplified implementation
	// Full RFC 5545 duration parsing would be more complex
	if value == "" {
		return 0
	}

	// Remove P prefix if present
	value = strings.TrimPrefix(value, "P")
	value = strings.TrimPrefix(value, "-P")

	// Simple parsing for common cases
	if strings.HasSuffix(value, "D") {
		days := 0
		if _, err := fmt.Sscanf(value, "%dD", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour
		}
	}

	if strings.Contains(value, "T") {
		// Handle time components
		parts := strings.Split(value, "T")
		if len(parts) == 2 {
			timePart := parts[1]

			hours := 0
			minutes := 0
			seconds := 0

			if strings.Contains(timePart, "H") {
				_, _ = fmt.Sscanf(timePart, "%dH", &hours)
			}
			if strings.Contains(timePart, "M") {
				_, _ = fmt.Sscanf(timePart, "%dM", &minutes)
			}
			if strings.Contains(timePart, "S") {
				_, _ = fmt.Sscanf(timePart, "%dS", &seconds)
			}

			return time.Duration(hours)*time.Hour +
				time.Duration(minutes)*time.Minute +
				time.Duration(seconds)*time.Second
		}
	}

	return 0
}
