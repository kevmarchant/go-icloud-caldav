package caldav

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CreateRecurringEvent creates a new recurring event with RRULE support.
func (c *CalDAVClient) CreateRecurringEvent(calendarPath string, event *CalendarObject, rrule string) error {
	return c.CreateRecurringEventWithContext(context.Background(), calendarPath, event, rrule)
}

// CreateRecurringEventWithContext creates a recurring event with the provided context.
func (c *CalDAVClient) CreateRecurringEventWithContext(ctx context.Context, calendarPath string, event *CalendarObject, rrule string) error {
	if rrule == "" {
		return fmt.Errorf("RRULE is required for recurring events")
	}

	parsedRule, err := ParseRRule(rrule)
	if err != nil {
		return fmt.Errorf("invalid RRULE: %w", err)
	}

	if parsedRule.Freq == "" {
		return fmt.Errorf("RRULE must specify FREQ")
	}

	event.RecurrenceRule = rrule

	return c.CreateEventWithContext(ctx, calendarPath, event)
}

// UpdateRecurrencePattern updates the recurrence rule of an existing event.
func (c *CalDAVClient) UpdateRecurrencePattern(calendarPath string, event *CalendarObject, newRRule string, etag string) error {
	return c.UpdateRecurrencePatternWithContext(context.Background(), calendarPath, event, newRRule, etag)
}

// UpdateRecurrencePatternWithContext updates recurrence pattern with the provided context.
func (c *CalDAVClient) UpdateRecurrencePatternWithContext(ctx context.Context, calendarPath string, event *CalendarObject, newRRule string, etag string) error {
	if newRRule != "" {
		parsedRule, err := ParseRRule(newRRule)
		if err != nil {
			return fmt.Errorf("invalid RRULE: %w", err)
		}

		if parsedRule.Freq == "" {
			return fmt.Errorf("RRULE must specify FREQ")
		}
	}

	event.RecurrenceRule = newRRule
	now := time.Now()
	event.LastModified = &now

	return c.UpdateEventWithContext(ctx, calendarPath, event, etag)
}

// DeleteRecurrenceInstance deletes a single occurrence of a recurring event.
// This creates an EXDATE entry for the specified occurrence.
func (c *CalDAVClient) DeleteRecurrenceInstance(calendarPath string, event *CalendarObject, instanceDate time.Time, etag string) error {
	return c.DeleteRecurrenceInstanceWithContext(context.Background(), calendarPath, event, instanceDate, etag)
}

// DeleteRecurrenceInstanceWithContext deletes a recurrence instance with the provided context.
func (c *CalDAVClient) DeleteRecurrenceInstanceWithContext(ctx context.Context, calendarPath string, event *CalendarObject, instanceDate time.Time, etag string) error {
	if event.RecurrenceRule == "" {
		return fmt.Errorf("event is not recurring")
	}

	if event.ExceptionDates == nil {
		event.ExceptionDates = []time.Time{}
	}

	for _, exDate := range event.ExceptionDates {
		if exDate.Equal(instanceDate) {
			return fmt.Errorf("instance already deleted")
		}
	}

	event.ExceptionDates = append(event.ExceptionDates, instanceDate)
	now := time.Now()
	event.LastModified = &now

	return c.UpdateEventWithContext(ctx, calendarPath, event, etag)
}

// UpdateRecurrenceInstance updates a single occurrence of a recurring event.
// This creates a detached instance with RECURRENCE-ID.
func (c *CalDAVClient) UpdateRecurrenceInstance(calendarPath string, instanceEvent *CalendarObject, recurrenceID time.Time) error {
	return c.UpdateRecurrenceInstanceWithContext(context.Background(), calendarPath, instanceEvent, recurrenceID)
}

// UpdateRecurrenceInstanceWithContext updates a recurrence instance with the provided context.
func (c *CalDAVClient) UpdateRecurrenceInstanceWithContext(ctx context.Context, calendarPath string, instanceEvent *CalendarObject, recurrenceID time.Time) error {
	if instanceEvent.UID == "" {
		return fmt.Errorf("instance must have the same UID as the recurring event")
	}

	instanceEvent.RecurrenceID = &recurrenceID
	instanceEvent.RecurrenceRule = ""

	if instanceEvent.UID == "" {
		instanceEvent.UID = generateUID()
	}

	return c.CreateEventWithContext(ctx, calendarPath, instanceEvent)
}

// ExpandRecurringEvent expands a recurring event into individual occurrences.
// Returns a list of CalendarObject instances for the specified time range.
func (c *CalDAVClient) ExpandRecurringEvent(event *CalendarObject, start, end time.Time) ([]*CalendarObject, error) {
	if event.RecurrenceRule == "" {
		return nil, fmt.Errorf("event is not recurring")
	}

	rrule, err := ParseRRule(event.RecurrenceRule)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RRULE: %w", err)
	}

	occurrences, err := ExpandRRule(rrule, event.StartTime, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to expand RRULE: %w", err)
	}

	var expandedEvents []*CalendarObject

	for _, occurrence := range occurrences {
		isException := false
		for _, exDate := range event.ExceptionDates {
			if occurrence.Equal(exDate) {
				isException = true
				break
			}
		}

		if isException {
			continue
		}

		duration := time.Duration(0)
		if event.EndTime != nil && event.StartTime != nil {
			duration = event.EndTime.Sub(*event.StartTime)
		}

		occurrenceCopy := occurrence
		endTime := occurrenceCopy.Add(duration)
		instanceEvent := &CalendarObject{
			UID:              event.UID,
			Summary:          event.Summary,
			Description:      event.Description,
			Location:         event.Location,
			StartTime:        &occurrenceCopy,
			EndTime:          &endTime,
			Organizer:        event.Organizer,
			Attendees:        event.Attendees,
			Categories:       event.Categories,
			Status:           event.Status,
			Class:            event.Class,
			Priority:         event.Priority,
			Transparency:     event.Transparency,
			URL:              event.URL,
			CustomProperties: event.CustomProperties,
			RecurrenceID:     &occurrenceCopy,
		}

		expandedEvents = append(expandedEvents, instanceEvent)
	}

	return expandedEvents, nil
}

// GetRecurringEvents retrieves all recurring events from a calendar.
func (c *CalDAVClient) GetRecurringEvents(calendarPath string) ([]*CalendarObject, error) {
	return c.GetRecurringEventsWithContext(context.Background(), calendarPath)
}

// GetRecurringEventsWithContext retrieves recurring events with the provided context.
func (c *CalDAVClient) GetRecurringEventsWithContext(ctx context.Context, calendarPath string) ([]*CalendarObject, error) {
	query := &CalendarQuery{
		Properties: []string{
			"UID",
			"SUMMARY",
			"DTSTART",
			"DTEND",
			"RRULE",
			"EXDATE",
			"RECURRENCE-ID",
		},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name: "RRULE",
						},
					},
				},
			},
		},
	}

	events, err := c.QueryCalendar(ctx, calendarPath, *query)
	if err != nil {
		return nil, fmt.Errorf("failed to query recurring events: %w", err)
	}

	recurringEvents := make([]*CalendarObject, 0)
	for i := range events {
		if events[i].RecurrenceRule != "" {
			recurringEvents = append(recurringEvents, &events[i])
		}
	}

	return recurringEvents, nil
}

// CreateRecurringEventWithExceptions creates a recurring event with predefined exceptions.
func (c *CalDAVClient) CreateRecurringEventWithExceptions(calendarPath string, event *CalendarObject, rrule string, exceptions []time.Time) error {
	return c.CreateRecurringEventWithExceptionsContext(context.Background(), calendarPath, event, rrule, exceptions)
}

// CreateRecurringEventWithExceptionsContext creates a recurring event with exceptions using the provided context.
func (c *CalDAVClient) CreateRecurringEventWithExceptionsContext(ctx context.Context, calendarPath string, event *CalendarObject, rrule string, exceptions []time.Time) error {
	if rrule == "" {
		return fmt.Errorf("RRULE is required for recurring events")
	}

	parsedRule, err := ParseRRule(rrule)
	if err != nil {
		return fmt.Errorf("invalid RRULE: %w", err)
	}

	if parsedRule.Freq == "" {
		return fmt.Errorf("RRULE must specify FREQ")
	}

	event.RecurrenceRule = rrule
	event.ExceptionDates = exceptions

	return c.CreateEventWithContext(ctx, calendarPath, event)
}

// ValidateRRule validates an RRULE string.
func ValidateRRule(rrule string) error {
	if rrule == "" {
		return fmt.Errorf("RRULE cannot be empty")
	}

	parsedRule, err := ParseRRule(rrule)
	if err != nil {
		return fmt.Errorf("failed to parse RRULE: %w", err)
	}

	if parsedRule.Freq == "" {
		return fmt.Errorf("RRULE must specify FREQ")
	}

	validFreq := []string{"DAILY", "WEEKLY", "MONTHLY", "YEARLY", "HOURLY", "MINUTELY", "SECONDLY"}
	isValidFreq := false
	for _, freq := range validFreq {
		if parsedRule.Freq == freq {
			isValidFreq = true
			break
		}
	}

	if !isValidFreq {
		return fmt.Errorf("invalid FREQ value: %s", parsedRule.Freq)
	}

	if parsedRule.Interval < 1 {
		return fmt.Errorf("INTERVAL must be >= 1")
	}

	if parsedRule.Count < 0 {
		return fmt.Errorf("COUNT must be >= 0")
	}

	if parsedRule.Count > 0 && parsedRule.Until != nil {
		return fmt.Errorf("COUNT and UNTIL cannot both be specified")
	}

	return nil
}

// BuildRRule builds an RRULE string from parameters.
func BuildRRule(freq string, interval int, count int, until *time.Time, byDay []string) string {
	parts := []string{fmt.Sprintf("FREQ=%s", strings.ToUpper(freq))}

	if interval > 1 {
		parts = append(parts, fmt.Sprintf("INTERVAL=%d", interval))
	}

	if count > 0 {
		parts = append(parts, fmt.Sprintf("COUNT=%d", count))
	} else if until != nil {
		parts = append(parts, fmt.Sprintf("UNTIL=%s", until.Format("20060102T150405Z")))
	}

	if len(byDay) > 0 {
		parts = append(parts, fmt.Sprintf("BYDAY=%s", strings.Join(byDay, ",")))
	}

	return strings.Join(parts, ";")
}
