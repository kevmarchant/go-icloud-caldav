package caldav

import (
	"context"
	"fmt"
	"io"
	"time"
)

// QueryCalendar performs a REPORT request on a calendar with a custom query.
// The calendarPath should be a calendar URL obtained from FindCalendars.
// Returns matching calendar objects (events, todos, etc.).
func (c *CalDAVClient) QueryCalendar(ctx context.Context, calendarPath string, query CalendarQuery) ([]CalendarObject, error) {
	xmlBody, err := buildCalendarQueryXML(query)
	if err != nil {
		return nil, fmt.Errorf("building calendar query XML: %w", err)
	}

	resp, err := c.report(ctx, calendarPath, xmlBody)
	if err != nil {
		return nil, fmt.Errorf("executing report request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 207 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, body)
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing multistatus response: %w", err)
	}

	objects := extractCalendarObjectsFromResponse(msResp)

	return objects, nil
}

// GetRecentEvents retrieves events within a specified number of days before and after today.
// For example, days=7 returns events from 7 days ago to 7 days in the future.
func (c *CalDAVClient) GetRecentEvents(ctx context.Context, calendarPath string, days int) ([]CalendarObject, error) {
	now := time.Now()
	startTime := now.AddDate(0, 0, -days)
	endTime := now.AddDate(0, 0, days)

	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		TimeRange: &TimeRange{
			Start: startTime,
			End:   endTime,
		},
	}

	return c.QueryCalendar(ctx, calendarPath, query)
}

// GetEventsByTimeRange retrieves all events within a specific time range.
// Returns events that occur between the start and end times.
func (c *CalDAVClient) GetEventsByTimeRange(ctx context.Context, calendarPath string, start, end time.Time) ([]CalendarObject, error) {
	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		TimeRange: &TimeRange{
			Start: start,
			End:   end,
		},
	}

	return c.QueryCalendar(ctx, calendarPath, query)
}

// GetEventByUID retrieves a specific event by its unique identifier.
// Returns an error if the event is not found.
func (c *CalDAVClient) GetEventByUID(ctx context.Context, calendarPath string, uid string) (*CalendarObject, error) {
	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		Filter: Filter{
			Component: "VEVENT",
			Props: []PropFilter{
				{
					Name: "UID",
					TextMatch: &TextMatch{
						Value: uid,
					},
				},
			},
		},
	}

	objects, err := c.QueryCalendar(ctx, calendarPath, query)
	if err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("event with UID %s not found", uid)
	}

	return &objects[0], nil
}

// CountEvents returns the number of events in a calendar.
// For efficiency, it queries a 4-year window (2 years past, 2 years future) from today.
func (c *CalDAVClient) CountEvents(ctx context.Context, calendarPath string) (int, error) {
	now := time.Now()
	startTime := now.AddDate(-2, 0, 0)
	endTime := now.AddDate(2, 0, 0)

	query := CalendarQuery{
		Properties: []string{"getetag"},
		Filter: Filter{
			Component: "VEVENT",
			TimeRange: &TimeRange{
				Start: startTime,
				End:   endTime,
			},
		},
	}

	objects, err := c.QueryCalendar(ctx, calendarPath, query)
	if err != nil {
		return 0, err
	}

	return len(objects), nil
}

// GetAllEvents retrieves all events from a calendar.
// Due to iCloud limitations, it queries a 4-year window (2 years past, 2 years future) from today.
func (c *CalDAVClient) GetAllEvents(ctx context.Context, calendarPath string) ([]CalendarObject, error) {
	now := time.Now()
	startTime := now.AddDate(-2, 0, 0)
	endTime := now.AddDate(2, 0, 0)

	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		Filter: Filter{
			Component: "VEVENT",
			TimeRange: &TimeRange{
				Start: startTime,
				End:   endTime,
			},
		},
	}

	return c.QueryCalendar(ctx, calendarPath, query)
}

// SearchEvents finds events whose summary contains the specified text.
// The search is case-insensitive.
func (c *CalDAVClient) SearchEvents(ctx context.Context, calendarPath string, searchText string) ([]CalendarObject, error) {
	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		Filter: Filter{
			Component: "VEVENT",
			Props: []PropFilter{
				{
					Name: "SUMMARY",
					TextMatch: &TextMatch{
						Value:     searchText,
						Collation: "i;ascii-casemap",
					},
				},
			},
		},
	}

	return c.QueryCalendar(ctx, calendarPath, query)
}

// GetUpcomingEvents retrieves future events from today up to 6 months ahead.
// If limit > 0, returns at most that many events.
func (c *CalDAVClient) GetUpcomingEvents(ctx context.Context, calendarPath string, limit int) ([]CalendarObject, error) {
	now := time.Now()
	endTime := now.AddDate(0, 6, 0)

	query := CalendarQuery{
		Properties: []string{"getetag", "calendar-data"},
		TimeRange: &TimeRange{
			Start: now,
			End:   endTime,
		},
	}

	objects, err := c.QueryCalendar(ctx, calendarPath, query)
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(objects) > limit {
		return objects[:limit], nil
	}

	return objects, nil
}
