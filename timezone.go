package caldav

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CreateTimeZoneInfo creates a TimeZoneInfo from a ParsedTimeZone.
func CreateTimeZoneInfo(tz ParsedTimeZone) *TimeZoneInfo {
	tzInfo := &TimeZoneInfo{
		TZID:        tz.TZID,
		Transitions: make([]TimeZoneTransition, 0),
	}

	// Create transitions from STANDARD and DAYLIGHT components
	if tz.StandardTime.TZOffsetTo != "" {
		transitions := createTransitionsFromComponent(tz.StandardTime, false)
		tzInfo.Transitions = append(tzInfo.Transitions, transitions...)
	}

	if tz.DaylightTime.TZOffsetTo != "" {
		transitions := createTransitionsFromComponent(tz.DaylightTime, true)
		tzInfo.Transitions = append(tzInfo.Transitions, transitions...)
	}

	// Try to create a Go time.Location if possible
	if loc, err := time.LoadLocation(tz.TZID); err == nil {
		tzInfo.Location = loc
	}

	return tzInfo
}

// createTransitionsFromComponent creates timezone transitions from a timezone component.
func createTransitionsFromComponent(component ParsedTimeZoneComponent, isDST bool) []TimeZoneTransition {
	transitions := make([]TimeZoneTransition, 0)

	offsetFrom := parseOffset(component.TZOffsetFrom)
	offsetTo := parseOffset(component.TZOffsetTo)

	// Create initial transition from DTSTART
	if component.DTStart != nil {
		transition := TimeZoneTransition{
			DateTime:     *component.DTStart,
			OffsetFrom:   offsetFrom,
			OffsetTo:     offsetTo,
			Abbreviation: component.TZName,
			IsDST:        isDST,
		}
		transitions = append(transitions, transition)
	}

	// Generate recurring transitions from RRULE
	if component.RecurrenceRule != "" && component.DTStart != nil {
		recurringTransitions := expandTimezoneTransitions(component, isDST, offsetFrom, offsetTo)
		transitions = append(transitions, recurringTransitions...)
	}

	// Add RDATE transitions
	for _, rdate := range component.RecurrenceDates {
		transition := TimeZoneTransition{
			DateTime:     rdate,
			OffsetFrom:   offsetFrom,
			OffsetTo:     offsetTo,
			Abbreviation: component.TZName,
			IsDST:        isDST,
		}
		transitions = append(transitions, transition)
	}

	return transitions
}

// parseOffset parses timezone offset string like "+0500" or "-0800" into time.Duration.
func parseOffset(offset string) time.Duration {
	if len(offset) < 3 {
		return 0
	}

	sign := 1
	if offset[0] == '-' {
		sign = -1
	}

	offset = strings.TrimPrefix(offset, "+")
	offset = strings.TrimPrefix(offset, "-")

	if len(offset) < 4 {
		return 0
	}

	hours, err := strconv.Atoi(offset[:2])
	if err != nil {
		return 0
	}

	minutes, err := strconv.Atoi(offset[2:4])
	if err != nil {
		return 0
	}

	seconds := 0
	if len(offset) >= 6 {
		if s, err := strconv.Atoi(offset[4:6]); err == nil {
			seconds = s
		}
	}

	duration := time.Duration(sign) * (time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second)

	return duration
}

// expandTimezoneTransitions generates recurring timezone transitions based on RRULE.
func expandTimezoneTransitions(component ParsedTimeZoneComponent, isDST bool,
	offsetFrom, offsetTo time.Duration) []TimeZoneTransition {

	transitions := make([]TimeZoneTransition, 0)

	if component.DTStart == nil || component.RecurrenceRule == "" {
		return transitions
	}

	// Generate transitions for the next 10 years from DTStart
	endTime := component.DTStart.AddDate(10, 0, 0)

	// Use the existing ExpandEvent logic but adapted for timezone transitions
	expanded, _ := ExpandEventWithExceptions(ParsedEvent{
		DTStart:        component.DTStart,
		RecurrenceRule: component.RecurrenceRule,
		ExceptionDates: component.ExceptionDates,
	}, make(map[string]*ParsedEvent), *component.DTStart, endTime)

	for _, occurrence := range expanded {
		if occurrence.DTStart == nil {
			continue
		}
		transition := TimeZoneTransition{
			DateTime:     *occurrence.DTStart,
			OffsetFrom:   offsetFrom,
			OffsetTo:     offsetTo,
			Abbreviation: component.TZName,
			IsDST:        isDST,
		}
		transitions = append(transitions, transition)
	}

	return transitions
}

// GetOffsetAtTime returns the timezone offset at a specific time.
func (tzi *TimeZoneInfo) GetOffsetAtTime(t time.Time) time.Duration {
	if len(tzi.Transitions) == 0 {
		return 0
	}

	// Find the most recent transition before or at the given time
	var mostRecentTransition *TimeZoneTransition
	for i := range tzi.Transitions {
		transition := &tzi.Transitions[i]
		if transition.DateTime.Before(t) || transition.DateTime.Equal(t) {
			if mostRecentTransition == nil || transition.DateTime.After(mostRecentTransition.DateTime) {
				mostRecentTransition = transition
			}
		}
	}

	if mostRecentTransition != nil {
		return mostRecentTransition.OffsetTo
	}

	// If no transition found before the time, use the first transition's offset
	return tzi.Transitions[0].OffsetFrom
}

// ConvertToUTC converts a local time to UTC using the timezone information.
func (tzi *TimeZoneInfo) ConvertToUTC(localTime time.Time) time.Time {
	offset := tzi.GetOffsetAtTime(localTime)
	return localTime.Add(-offset)
}

// ConvertFromUTC converts a UTC time to local time using the timezone information.
func (tzi *TimeZoneInfo) ConvertFromUTC(utcTime time.Time) time.Time {
	offset := tzi.GetOffsetAtTime(utcTime)
	return utcTime.Add(offset)
}

// IsDSTAtTime returns whether DST is active at the given time.
func (tzi *TimeZoneInfo) IsDSTAtTime(t time.Time) bool {
	if len(tzi.Transitions) == 0 {
		return false
	}

	// Find the most recent transition before or at the given time
	var mostRecentTransition *TimeZoneTransition
	for i := range tzi.Transitions {
		transition := &tzi.Transitions[i]
		if transition.DateTime.Before(t) || transition.DateTime.Equal(t) {
			if mostRecentTransition == nil || transition.DateTime.After(mostRecentTransition.DateTime) {
				mostRecentTransition = transition
			}
		}
	}

	return mostRecentTransition != nil && mostRecentTransition.IsDST
}

// GetTZIDFromGoLocation attempts to find a TZID string from a Go time.Location.
func GetTZIDFromGoLocation(loc *time.Location) string {
	if loc == nil {
		return ""
	}
	return loc.String()
}

// LoadLocationFromTZID attempts to load a Go time.Location from a TZID string.
func LoadLocationFromTZID(tzid string) (*time.Location, error) {
	if tzid == "" {
		return nil, fmt.Errorf("cannot load timezone for empty TZID")
	}

	// Try direct loading first
	if loc, err := time.LoadLocation(tzid); err == nil {
		return loc, nil
	}

	// Try common TZID mappings
	mappings := map[string]string{
		"US/Eastern":    "America/New_York",
		"US/Central":    "America/Chicago",
		"US/Mountain":   "America/Denver",
		"US/Pacific":    "America/Los_Angeles",
		"Europe/London": "Europe/London",
		"Europe/Paris":  "Europe/Paris",
		"Asia/Tokyo":    "Asia/Tokyo",
	}

	if mapped, exists := mappings[tzid]; exists {
		if loc, err := time.LoadLocation(mapped); err == nil {
			return loc, nil
		}
	}

	return nil, fmt.Errorf("cannot load timezone for TZID: %s", tzid)
}
