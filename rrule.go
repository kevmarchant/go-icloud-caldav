package caldav

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RRule represents a parsed recurrence rule
type RRule struct {
	Freq       string
	Interval   int
	Count      int
	Until      *time.Time
	ByDay      []string
	ByMonth    []int
	ByMonthDay []int
	BySetPos   []int
	ByWeekNo   []int
	ByYearDay  []int
	ByHour     []int
	ByMinute   []int
	BySecond   []int
	WeekStart  string
}

// ParseRRule parses an RRULE string into a structured RRule
func ParseRRule(rruleStr string) (*RRule, error) {
	if rruleStr == "" {
		return nil, nil
	}

	rule := &RRule{
		Interval: 1,
	}

	parts := strings.Split(rruleStr, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key := strings.ToUpper(kv[0])
		value := kv[1]

		switch key {
		case "FREQ":
			rule.Freq = value
		case "INTERVAL":
			if i, err := strconv.Atoi(value); err == nil {
				rule.Interval = i
			}
		case "COUNT":
			if c, err := strconv.Atoi(value); err == nil {
				rule.Count = c
			}
		case "UNTIL":
			if t, err := parseRRuleTime(value); err == nil {
				rule.Until = &t
			}
		case "BYDAY":
			rule.ByDay = strings.Split(value, ",")
		case "BYMONTH":
			rule.ByMonth = parseIntList(value)
		case "BYMONTHDAY":
			rule.ByMonthDay = parseIntList(value)
		case "BYSETPOS":
			rule.BySetPos = parseIntList(value)
		case "BYWEEKNO":
			rule.ByWeekNo = parseIntList(value)
		case "BYYEARDAY":
			rule.ByYearDay = parseIntList(value)
		case "BYHOUR":
			rule.ByHour = parseIntList(value)
		case "BYMINUTE":
			rule.ByMinute = parseIntList(value)
		case "BYSECOND":
			rule.BySecond = parseIntList(value)
		case "WKST":
			rule.WeekStart = value
		}
	}

	return rule, nil
}

// ExpandEvent generates individual event occurrences from a recurring event
func ExpandEvent(event ParsedEvent, start, end time.Time) ([]ParsedEvent, error) {
	if event.RecurrenceRule == "" && len(event.RecurrenceDates) == 0 {
		return []ParsedEvent{event}, nil
	}

	occurrences := []ParsedEvent{}
	occurrenceMap := make(map[string]bool)
	excludeMap := make(map[string]bool)

	var duration time.Duration
	if event.DTEnd != nil && event.DTStart != nil {
		duration = event.DTEnd.Sub(*event.DTStart)
	}

	// Build exclusion map from EXRULE if present
	if event.ExceptionRule != "" {
		exRule, err := ParseRRule(event.ExceptionRule)
		if err == nil && exRule != nil && event.DTStart != nil {
			current := *event.DTStart
			maxIterations := 10000
			iteration := 0

			for iteration < maxIterations {
				iteration++
				if current.After(end.AddDate(1, 0, 0)) {
					break
				}
				if exRule.Until != nil && current.After(*exRule.Until) {
					break
				}
				excludeMap[current.Format("20060102T150405Z")] = true
				current = nextOccurrence(current, exRule)
			}
		}
	}

	// Process RRULE occurrences
	if event.RecurrenceRule != "" {
		rule, err := ParseRRule(event.RecurrenceRule)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RRULE: %w", err)
		}

		if rule != nil && event.DTStart != nil {
			current := *event.DTStart
			instanceCount := 0
			maxIterations := 10000
			iteration := 0
			isFirstOccurrence := true

			for iteration < maxIterations {
				iteration++

				if current.After(end) {
					break
				}

				if rule.Until != nil && current.After(*rule.Until) {
					break
				}

				if rule.Count > 0 && instanceCount >= rule.Count {
					break
				}

				// For complex patterns, check if current matches the pattern
				shouldInclude := true
				if rule.Freq == "MONTHLY" && len(rule.ByDay) > 0 && isFirstOccurrence {
					// Check if the start date actually matches the BYDAY pattern
					shouldInclude = false
					byDayParts := parseByDay(rule.ByDay)

					// If we have BYSETPOS, we need to check differently
					if len(rule.BySetPos) > 0 {
						// Generate all matching dates in the current month
						var monthDates []time.Time
						for _, part := range byDayParts {
							weekday := weekdayToInt(part.Weekday)
							if part.Position == 0 {
								// Every occurrence of this weekday in the month
								for day := 1; day <= 31; day++ {
									date := time.Date(current.Year(), current.Month(), day, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
									if date.Month() != current.Month() {
										break
									}
									if date.Weekday() == weekday {
										monthDates = append(monthDates, date)
									}
								}
							}
						}

						// Apply BYSETPOS
						if len(monthDates) > 0 {
							sortTimes(monthDates)
							filteredDates := applyBySetPos(monthDates, rule.BySetPos)
							for _, date := range filteredDates {
								if date.Day() == current.Day() {
									shouldInclude = true
									break
								}
							}
						}
					} else {
						// Regular BYDAY check
						for _, part := range byDayParts {
							if part.Position != 0 {
								// Check specific position
								if date, ok := getNthWeekdayInMonth(current.Year(), current.Month(), weekdayToInt(part.Weekday), part.Position); ok {
									if date.Day() == current.Day() {
										shouldInclude = true
										break
									}
								}
							} else {
								// Any occurrence of this weekday
								if current.Weekday() == weekdayToInt(part.Weekday) {
									shouldInclude = true
									break
								}
							}
						}
					}
				}

				if shouldInclude && !current.Before(start) && !current.After(end) {
					// Expand for time-level granularity if needed
					timesForDay := expandTimeGranularity(current, rule)

					for _, expandedTime := range timesForDay {
						// Check if we've hit the COUNT limit
						if rule.Count > 0 && len(occurrences) >= rule.Count {
							break
						}

						key := expandedTime.Format("20060102T150405Z")
						if !occurrenceMap[key] && !excludeMap[key] {
							occurrence := event
							occTime := expandedTime
							occurrence.DTStart = &occTime

							if event.DTEnd != nil {
								endTime := expandedTime.Add(duration)
								occurrence.DTEnd = &endTime
							}

							recID := expandedTime
							occurrence.RecurrenceID = &recID

							occurrences = append(occurrences, occurrence)
							occurrenceMap[key] = true
						}
					}
				}

				instanceCount++
				isFirstOccurrence = false
				current = nextOccurrence(current, rule)

				// Check if we've hit the COUNT limit with expansions
				if rule.Count > 0 && len(occurrences) >= rule.Count {
					break
				}
			}
		}
	}

	// Add RDATE occurrences
	for _, rdate := range event.RecurrenceDates {
		// Check if this date is in the exception dates (EXDATE)
		isExcluded := false
		for _, exDate := range event.ExceptionDates {
			if isSameDateTime(rdate, exDate) {
				isExcluded = true
				break
			}
		}

		if !isExcluded && !rdate.Before(start) && !rdate.After(end) {
			key := rdate.Format("20060102T150405Z")
			if !occurrenceMap[key] {
				occurrence := event
				rdateCopy := rdate
				occurrence.DTStart = &rdateCopy

				if event.DTEnd != nil && event.DTStart != nil {
					endTime := rdate.Add(duration)
					occurrence.DTEnd = &endTime
				}

				occurrence.RecurrenceID = &rdateCopy

				occurrences = append(occurrences, occurrence)
				occurrenceMap[key] = true
			}
		}
	}

	// If no recurrence but we have the base event
	if len(occurrences) == 0 && event.DTStart != nil {
		if !event.DTStart.Before(start) && !event.DTStart.After(end) {
			return []ParsedEvent{event}, nil
		}
	}

	return occurrences, nil
}

// nextOccurrence calculates the next occurrence based on the recurrence rule
func nextOccurrence(current time.Time, rule *RRule) time.Time {
	// For complex patterns with BYDAY and positions, we need different logic
	if rule.Freq == "MONTHLY" && len(rule.ByDay) > 0 {
		return nextMonthlyByDayOccurrence(current, rule)
	}

	if rule.Freq == "YEARLY" && len(rule.ByWeekNo) > 0 {
		return nextYearlyByWeekNoOccurrence(current, rule)
	}

	// Simple frequency-based advancement
	switch rule.Freq {
	case "DAILY":
		return current.AddDate(0, 0, rule.Interval)
	case "WEEKLY":
		return nextWeeklyOccurrence(current, rule)
	case "MONTHLY":
		return nextMonthlyOccurrence(current, rule)
	case "YEARLY":
		return current.AddDate(rule.Interval, 0, 0)
	default:
		return current.AddDate(0, 0, 1)
	}
}

// nextWeeklyOccurrence handles weekly recurrence with BYDAY and WKST support
func nextWeeklyOccurrence(current time.Time, rule *RRule) time.Time {
	if len(rule.ByDay) == 0 {
		return current.AddDate(0, 0, 7*rule.Interval)
	}

	// Parse BYDAY values
	byDayParts := parseByDay(rule.ByDay)
	if len(byDayParts) == 0 {
		return current.AddDate(0, 0, 7*rule.Interval)
	}

	// Find next matching weekday
	for days := 1; days <= 7*rule.Interval; days++ {
		candidate := current.AddDate(0, 0, days)
		candidateWeekday := candidate.Weekday()

		for _, part := range byDayParts {
			if weekdayToInt(part.Weekday) == candidateWeekday {
				return candidate
			}
		}
	}

	// Fallback
	return current.AddDate(0, 0, 7*rule.Interval)
}

// nextMonthlyOccurrence handles monthly recurrence
func nextMonthlyOccurrence(current time.Time, rule *RRule) time.Time {
	if len(rule.ByMonthDay) > 0 {
		// Handle BYMONTHDAY
		return nextMonthlyByMonthDayOccurrence(current, rule)
	}

	// Simple monthly increment
	return current.AddDate(0, rule.Interval, 0)
}

// nextMonthlyByDayOccurrence handles MONTHLY with BYDAY (e.g., 2nd Tuesday)
func nextMonthlyByDayOccurrence(current time.Time, rule *RRule) time.Time {
	byDayParts := parseByDay(rule.ByDay)
	if len(byDayParts) == 0 {
		return current.AddDate(0, rule.Interval, 0)
	}

	// Start checking from next day
	candidate := current.AddDate(0, 0, 1)
	maxMonths := 100 // Safety limit

	for months := 0; months < maxMonths; months++ {
		year := candidate.Year()
		month := candidate.Month()

		// Generate all possible dates in this month based on BYDAY
		var monthDates []time.Time
		for _, part := range byDayParts {
			weekday := weekdayToInt(part.Weekday)

			if part.Position == 0 {
				// Every occurrence of this weekday in the month
				for day := 1; day <= 31; day++ {
					date := time.Date(year, month, day, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
					if date.Month() != month {
						break // Went to next month
					}
					if date.Weekday() == weekday && date.After(current) {
						monthDates = append(monthDates, date)
					}
				}
			} else {
				// Specific position (e.g., 2nd Tuesday, last Friday)
				if date, ok := getNthWeekdayInMonth(year, month, weekday, part.Position); ok {
					if date.After(current) {
						withTime := time.Date(date.Year(), date.Month(), date.Day(),
							current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
						monthDates = append(monthDates, withTime)
					}
				}
			}
		}

		// Apply BYSETPOS if specified
		if len(rule.BySetPos) > 0 && len(monthDates) > 0 {
			// Sort dates
			sortTimes(monthDates)
			monthDates = applyBySetPos(monthDates, rule.BySetPos)
		}

		// Return first valid date
		if len(monthDates) > 0 {
			sortTimes(monthDates)
			return monthDates[0]
		}

		// Move to next month interval
		candidate = time.Date(year, month+time.Month(rule.Interval), 1,
			current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
	}

	// Fallback
	return current.AddDate(0, rule.Interval, 0)
}

// nextMonthlyByMonthDayOccurrence handles MONTHLY with BYMONTHDAY
func nextMonthlyByMonthDayOccurrence(current time.Time, rule *RRule) time.Time {
	// Find next matching day
	candidate := current.AddDate(0, 0, 1)
	maxMonths := 100 // Safety limit

	for months := 0; months < maxMonths; months++ {
		year := candidate.Year()
		month := candidate.Month()

		for _, dayNum := range rule.ByMonthDay {
			var targetDay int
			if dayNum > 0 {
				targetDay = dayNum
			} else {
				// Negative day number (from end of month)
				lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
				targetDay = lastDay + dayNum + 1
			}

			if targetDay >= 1 && targetDay <= 31 {
				date := time.Date(year, month, targetDay, current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
				if date.Month() == month && date.After(current) {
					return date
				}
			}
		}

		// Move to next month interval
		candidate = time.Date(year, month+time.Month(rule.Interval), 1,
			current.Hour(), current.Minute(), current.Second(), current.Nanosecond(), current.Location())
	}

	// Fallback
	return current.AddDate(0, rule.Interval, 0)
}

// nextYearlyByWeekNoOccurrence handles YEARLY with BYWEEKNO
func nextYearlyByWeekNoOccurrence(current time.Time, rule *RRule) time.Time {
	// This is complex and involves ISO week calculations
	// For now, simple fallback
	return current.AddDate(rule.Interval, 0, 0)
}

// sortTimes sorts a slice of time.Time values
func sortTimes(times []time.Time) {
	for i := 0; i < len(times)-1; i++ {
		for j := i + 1; j < len(times); j++ {
			if times[j].Before(times[i]) {
				times[i], times[j] = times[j], times[i]
			}
		}
	}
}

// parseRRuleTime parses time strings in RRULE format
func parseRRuleTime(s string) (time.Time, error) {
	return ParseCalDAVTime(s)
}

// parseIntList parses a comma-separated list of integers
func parseIntList(s string) []int {
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		if i, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			result = append(result, i)
		}
	}

	return result
}

// EventException represents a modified instance of a recurring event
type EventException struct {
	RecurrenceID *time.Time
	Event        ParsedEvent
}

// ExpandEvents expands all recurring events in a ParsedCalendarData
func ExpandEvents(data *ParsedCalendarData, start, end time.Time) (*ParsedCalendarData, error) {
	if data == nil {
		return nil, nil
	}

	expandedData := &ParsedCalendarData{
		Version:          data.Version,
		ProdID:           data.ProdID,
		CalScale:         data.CalScale,
		Method:           data.Method,
		Events:           []ParsedEvent{},
		Todos:            data.Todos,
		FreeBusy:         data.FreeBusy,
		TimeZones:        data.TimeZones,
		Alarms:           data.Alarms,
		CustomProperties: data.CustomProperties,
	}

	// Group events by UID to handle exceptions
	eventsByUID := make(map[string][]ParsedEvent)
	for _, event := range data.Events {
		eventsByUID[event.UID] = append(eventsByUID[event.UID], event)
	}

	// Process each unique event
	for _, events := range eventsByUID {
		if len(events) == 1 && events[0].RecurrenceRule == "" {
			// Simple non-recurring event
			expandedData.Events = append(expandedData.Events, events[0])
			continue
		}

		// Find the master event and exceptions
		var masterEvent *ParsedEvent
		exceptions := make(map[string]*ParsedEvent)

		for i := range events {
			if events[i].RecurrenceID == nil && events[i].RecurrenceRule != "" {
				masterEvent = &events[i]
			} else if events[i].RecurrenceID != nil {
				// This is an exception/modification
				recIDStr := events[i].RecurrenceID.Format("20060102T150405Z")
				exceptions[recIDStr] = &events[i]
			}
		}

		if masterEvent != nil {
			// Expand the master event
			expanded, err := ExpandEventWithExceptions(*masterEvent, exceptions, start, end)
			if err != nil {
				// On error, just add original events
				expandedData.Events = append(expandedData.Events, events...)
				continue
			}
			expandedData.Events = append(expandedData.Events, expanded...)
		} else {
			// No master event found, just add all events
			expandedData.Events = append(expandedData.Events, events...)
		}
	}

	return expandedData, nil
}

// ExpandEventWithExceptions expands a recurring event while applying exceptions
func ExpandEventWithExceptions(event ParsedEvent, exceptions map[string]*ParsedEvent, start, end time.Time) ([]ParsedEvent, error) {
	if event.RecurrenceRule == "" && len(event.RecurrenceDates) == 0 {
		return []ParsedEvent{event}, nil
	}

	occurrences := []ParsedEvent{}
	occurrenceMap := make(map[string]bool)

	var duration time.Duration
	if event.DTEnd != nil && event.DTStart != nil {
		duration = event.DTEnd.Sub(*event.DTStart)
	}

	// Process RRULE occurrences
	if event.RecurrenceRule != "" {
		rule, err := ParseRRule(event.RecurrenceRule)
		if err != nil {
			return nil, fmt.Errorf("failed to parse RRULE: %w", err)
		}

		if rule != nil && event.DTStart != nil {
			current := *event.DTStart
			instanceCount := 0
			maxIterations := 10000
			iteration := 0

			for iteration < maxIterations {
				iteration++

				if current.After(end) {
					break
				}

				if rule.Until != nil && current.After(*rule.Until) {
					break
				}

				if rule.Count > 0 && instanceCount >= rule.Count {
					break
				}

				// Check if this date is in the exception dates (EXDATE)
				isExcluded := false
				for _, exDate := range event.ExceptionDates {
					if isSameDateTime(current, exDate) {
						isExcluded = true
						break
					}
				}

				if !isExcluded && !current.Before(start) && !current.After(end) {
					recIDStr := current.Format("20060102T150405Z")
					if !occurrenceMap[recIDStr] {
						// Check if there's an exception/modification for this occurrence
						if exception, exists := exceptions[recIDStr]; exists {
							// Use the modified version
							occurrences = append(occurrences, *exception)
						} else {
							// Create normal occurrence
							occurrence := event
							occTime := current
							occurrence.DTStart = &occTime

							if event.DTEnd != nil {
								endTime := current.Add(duration)
								occurrence.DTEnd = &endTime
							}

							recID := current
							occurrence.RecurrenceID = &recID

							occurrences = append(occurrences, occurrence)
						}
						occurrenceMap[recIDStr] = true
					}
				}

				instanceCount++
				current = nextOccurrence(current, rule)
			}
		}
	}

	// Process RDATE occurrences
	for _, rdate := range event.RecurrenceDates {
		// Check if this date is in the exception dates (EXDATE)
		isExcluded := false
		for _, exDate := range event.ExceptionDates {
			if isSameDateTime(rdate, exDate) {
				isExcluded = true
				break
			}
		}

		if !isExcluded && !rdate.Before(start) && !rdate.After(end) {
			recIDStr := rdate.Format("20060102T150405Z")
			if !occurrenceMap[recIDStr] {
				// Check if there's an exception/modification for this occurrence
				if exception, exists := exceptions[recIDStr]; exists {
					// Use the modified version
					occurrences = append(occurrences, *exception)
				} else {
					// Create occurrence from RDATE
					occurrence := event
					rdateCopy := rdate
					occurrence.DTStart = &rdateCopy

					if event.DTEnd != nil && event.DTStart != nil {
						endTime := rdate.Add(duration)
						occurrence.DTEnd = &endTime
					}

					occurrence.RecurrenceID = &rdateCopy

					occurrences = append(occurrences, occurrence)
				}
				occurrenceMap[recIDStr] = true
			}
		}
	}

	// If no recurrence but we have the base event
	if len(occurrences) == 0 && event.DTStart != nil {
		if !event.DTStart.Before(start) && !event.DTStart.After(end) {
			return []ParsedEvent{event}, nil
		}
	}

	return occurrences, nil
}

// isSameDateTime checks if two times represent the same date/time
func isSameDateTime(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() &&
		t1.Month() == t2.Month() &&
		t1.Day() == t2.Day() &&
		t1.Hour() == t2.Hour() &&
		t1.Minute() == t2.Minute()
}

// ByDayPart represents a parsed BYDAY value with optional position
type ByDayPart struct {
	Position int    // 0 means every, positive for nth, negative for nth from end
	Weekday  string // MO, TU, WE, TH, FR, SA, SU
}

// parseByDay parses BYDAY values that may have position prefixes
func parseByDay(byDayList []string) []ByDayPart {
	var result []ByDayPart
	weekdays := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

	for _, dayStr := range byDayList {
		dayStr = strings.TrimSpace(dayStr)
		var part ByDayPart

		// Check if there's a position prefix
		foundWeekday := false
		for _, wd := range weekdays {
			if strings.HasSuffix(dayStr, wd) {
				part.Weekday = wd
				prefix := strings.TrimSuffix(dayStr, wd)
				if prefix != "" {
					if pos, err := strconv.Atoi(prefix); err == nil {
						part.Position = pos
					}
				}
				foundWeekday = true
				break
			}
		}

		if foundWeekday {
			result = append(result, part)
		}
	}

	return result
}

// weekdayToInt converts weekday string to time.Weekday
func weekdayToInt(wd string) time.Weekday {
	switch wd {
	case "SU":
		return time.Sunday
	case "MO":
		return time.Monday
	case "TU":
		return time.Tuesday
	case "WE":
		return time.Wednesday
	case "TH":
		return time.Thursday
	case "FR":
		return time.Friday
	case "SA":
		return time.Saturday
	default:
		return time.Sunday
	}
}

// getNthWeekdayInMonth finds the nth occurrence of a weekday in a month
func getNthWeekdayInMonth(year int, month time.Month, weekday time.Weekday, n int) (time.Time, bool) {
	if n == 0 {
		return time.Time{}, false
	}

	if n > 0 {
		// Count from beginning of month
		firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

		// Find first occurrence of the weekday
		daysUntilWeekday := (int(weekday) - int(firstOfMonth.Weekday()) + 7) % 7
		firstOccurrence := firstOfMonth.AddDate(0, 0, daysUntilWeekday)

		// Add weeks to get to nth occurrence
		targetDate := firstOccurrence.AddDate(0, 0, (n-1)*7)

		// Check if still in same month
		if targetDate.Month() != month {
			return time.Time{}, false
		}

		return targetDate, true
	} else {
		// Count from end of month
		nextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
		lastOfMonth := nextMonth.AddDate(0, 0, -1)

		// Find last occurrence of the weekday
		daysBackToWeekday := (int(lastOfMonth.Weekday()) - int(weekday) + 7) % 7
		lastOccurrence := lastOfMonth.AddDate(0, 0, -daysBackToWeekday)

		// Go back weeks for negative position
		targetDate := lastOccurrence.AddDate(0, 0, (n+1)*7)

		// Check if still in same month
		if targetDate.Month() != month {
			return time.Time{}, false
		}

		return targetDate, true
	}
}

// expandTimeGranularity expands occurrences for BYHOUR, BYMINUTE, BYSECOND
func expandTimeGranularity(baseTime time.Time, rule *RRule) []time.Time {
	var times []time.Time

	// If no time-level constraints, return original
	if len(rule.ByHour) == 0 && len(rule.ByMinute) == 0 && len(rule.BySecond) == 0 {
		return []time.Time{baseTime}
	}

	hours := rule.ByHour
	if len(hours) == 0 {
		hours = []int{baseTime.Hour()}
	}

	minutes := rule.ByMinute
	if len(minutes) == 0 {
		minutes = []int{baseTime.Minute()}
	}

	seconds := rule.BySecond
	if len(seconds) == 0 {
		seconds = []int{baseTime.Second()}
	}

	for _, h := range hours {
		if h < 0 || h > 23 {
			continue
		}
		for _, m := range minutes {
			if m < 0 || m > 59 {
				continue
			}
			for _, s := range seconds {
				if s < 0 || s > 59 {
					continue
				}
				t := time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(),
					h, m, s, baseTime.Nanosecond(), baseTime.Location())
				times = append(times, t)
			}
		}
	}

	return times
}

// applyBySetPos filters a set of occurrences according to BYSETPOS
func applyBySetPos(occurrences []time.Time, positions []int) []time.Time {
	if len(positions) == 0 || len(occurrences) == 0 {
		return occurrences
	}

	var result []time.Time
	for _, pos := range positions {
		var idx int
		if pos > 0 {
			idx = pos - 1 // Convert to 0-based index
		} else if pos < 0 {
			idx = len(occurrences) + pos // Negative index from end
		} else {
			continue // pos == 0 is invalid
		}

		if idx >= 0 && idx < len(occurrences) {
			result = append(result, occurrences[idx])
		}
	}

	return result
}
