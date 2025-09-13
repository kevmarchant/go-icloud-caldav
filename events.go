package caldav

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// QueryCalendar performs a REPORT request on a calendar with a custom query.
// The calendarPath should be a calendar URL obtained from FindCalendars.
// Returns matching calendar objects (events, todos, etc.).
func (c *CalDAVClient) QueryCalendar(ctx context.Context, calendarPath string, query CalendarQuery) ([]CalendarObject, error) {
	xmlBody, err := buildCalendarQueryXML(query)
	if err != nil {
		return nil, wrapErrorWithType("query.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.report(ctx, calendarPath, xmlBody)
	if err != nil {
		return nil, wrapError("query.execute", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		body, _ := io.ReadAll(resp.Body)
		return nil, newCalDAVError("query", resp.StatusCode, string(body))
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("query.parse", ErrorTypeInvalidResponse, err)
	}

	objects := extractCalendarObjectsFromResponseWithOptions(msResp, c.autoParsing)

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
		return nil, newTypedErrorWithContext("event.byuid", ErrorTypeNotFound, "event not found", ErrNotFound, map[string]interface{}{"uid": uid})
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

// TextCollation specifies the collation algorithm for text matching.
type TextCollation string

const (
	CollationASCIICaseMap TextCollation = "i;ascii-casemap"
	CollationOctet        TextCollation = "i;octet"
	CollationUnicode      TextCollation = "i;unicode-casemap"
)

// LogicalOperator represents boolean logic operators for complex queries.
type LogicalOperator string

const (
	OperatorAND LogicalOperator = "AND"
	OperatorOR  LogicalOperator = "OR"
	OperatorNOT LogicalOperator = "NOT"
)

// ParameterFilter filters properties by their parameter values.
type ParameterFilter struct {
	ParameterName  string
	ParameterValue string
	TextMatch      string
	Collation      TextCollation
	NegateMatch    bool
}

// AdvancedCalendarQuery extends CalendarQuery with advanced filtering capabilities.
type AdvancedCalendarQuery struct {
	Properties       []string
	Filter           Filter
	TimeRange        *TimeRange
	TextCollation    TextCollation
	ParameterFilters []ParameterFilter
	LogicalOperator  LogicalOperator
	NestedQueries    []AdvancedCalendarQuery
}

// PropertyParameterMatch filters events by property parameter values.
type PropertyParameterMatch struct {
	PropertyName   string
	ParameterName  string
	ParameterValue string
	MatchType      string
}

// ComplexFilter represents a complex boolean filter with nested conditions.
type ComplexFilter struct {
	Operator   LogicalOperator
	Conditions []FilterCondition
}

// FilterCondition represents a single condition in a complex filter.
type FilterCondition struct {
	PropertyName   string
	PropertyValue  string
	ParameterName  string
	ParameterValue string
	Operator       string
	Collation      TextCollation
	Negate         bool
}

// QueryWithTextCollation performs a calendar query with text collation options.
func (c *CalDAVClient) QueryWithTextCollation(ctx context.Context, calendarHref string, query AdvancedCalendarQuery) ([]CalendarObject, error) {
	xmlBody, err := buildAdvancedQueryXML(query)
	if err != nil {
		return nil, wrapErrorWithType("advanced_query.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.report(ctx, calendarHref, []byte(xmlBody))
	if err != nil {
		return nil, wrapErrorWithType("advanced_query.request", ErrorTypeNetwork, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		return nil, newTypedError("advanced_query.status", ErrorTypeInvalidResponse,
			fmt.Sprintf("expected status 207, got %d", resp.StatusCode), nil)
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("advanced_query.parse", ErrorTypeInvalidResponse, err)
	}

	var objects []CalendarObject
	for _, r := range msResp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 && ps.Prop.CalendarData != "" {
				obj := CalendarObject{
					Href:         r.Href,
					ETag:         ps.Prop.ETag,
					CalendarData: ps.Prop.CalendarData,
				}
				objects = append(objects, obj)
			}
		}
	}

	return objects, nil
}

// QueryByAttendeeStatus filters events by attendee participation status.
func (c *CalDAVClient) QueryByAttendeeStatus(ctx context.Context, calendarHref string, attendeeEmail string, partstat string) ([]CalendarObject, error) {
	paramFilter := ParameterFilter{
		ParameterName:  "PARTSTAT",
		ParameterValue: partstat,
		TextMatch:      attendeeEmail,
		Collation:      CollationASCIICaseMap,
	}

	query := AdvancedCalendarQuery{
		Properties: []string{"calendar-data"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name:      "ATTENDEE",
							TextMatch: &TextMatch{Value: attendeeEmail},
						},
					},
				},
			},
		},
		ParameterFilters: []ParameterFilter{paramFilter},
		TextCollation:    CollationASCIICaseMap,
	}

	return c.QueryWithTextCollation(ctx, calendarHref, query)
}

// QueryWithComplexFilter performs a query with complex boolean logic.
func (c *CalDAVClient) QueryWithComplexFilter(ctx context.Context, calendarHref string, filter ComplexFilter) ([]CalendarObject, error) {
	query := buildComplexQuery(filter)
	return c.QueryWithTextCollation(ctx, calendarHref, query)
}

// FindEventsWithParameterMatch finds events matching specific parameter values.
func (c *CalDAVClient) FindEventsWithParameterMatch(ctx context.Context, calendarHref string, matches []PropertyParameterMatch) ([]CalendarObject, error) {
	var propFilters []PropFilter
	for _, match := range matches {
		pf := PropFilter{
			Name: match.PropertyName,
		}
		if match.ParameterValue != "" {
			pf.TextMatch = &TextMatch{
				Value:     match.ParameterValue,
				Collation: string(CollationASCIICaseMap),
			}
		}
		propFilters = append(propFilters, pf)
	}

	query := CalendarQuery{
		Properties: []string{"calendar-data", "getetag"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props:     propFilters,
				},
			},
		},
	}

	return c.QueryCalendar(ctx, calendarHref, query)
}

// SearchEventsByText performs a text search across all event properties.
func (c *CalDAVClient) SearchEventsByText(ctx context.Context, calendarHref string, searchText string, collation TextCollation) ([]CalendarObject, error) {
	if collation == "" {
		collation = CollationASCIICaseMap
	}

	commonProps := []string{"SUMMARY", "DESCRIPTION", "LOCATION", "COMMENT", "CONTACT"}
	var propFilters []PropFilter

	for _, propName := range commonProps {
		propFilters = append(propFilters, PropFilter{
			Name: propName,
			TextMatch: &TextMatch{
				Value:     searchText,
				Collation: string(collation),
			},
		})
	}

	query := AdvancedCalendarQuery{
		Properties: []string{"calendar-data", "getetag"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props:     propFilters,
				},
			},
		},
		TextCollation:   collation,
		LogicalOperator: OperatorOR,
	}

	return c.QueryWithTextCollation(ctx, calendarHref, query)
}

// QueryByOrganizer finds events organized by a specific user.
func (c *CalDAVClient) QueryByOrganizer(ctx context.Context, calendarHref string, organizerEmail string) ([]CalendarObject, error) {
	query := CalendarQuery{
		Properties: []string{"calendar-data"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name: "ORGANIZER",
							TextMatch: &TextMatch{
								Value:     organizerEmail,
								Collation: string(CollationASCIICaseMap),
							},
						},
					},
				},
			},
		},
	}

	return c.QueryCalendar(ctx, calendarHref, query)
}

// QueryByCategory finds events with specific categories.
func (c *CalDAVClient) QueryByCategory(ctx context.Context, calendarHref string, categories []string) ([]CalendarObject, error) {
	var propFilters []PropFilter
	for _, category := range categories {
		propFilters = append(propFilters, PropFilter{
			Name: "CATEGORIES",
			TextMatch: &TextMatch{
				Value:     category,
				Collation: string(CollationASCIICaseMap),
			},
		})
	}

	query := AdvancedCalendarQuery{
		Properties: []string{"calendar-data"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props:     propFilters,
				},
			},
		},
		LogicalOperator: OperatorOR,
		TextCollation:   CollationASCIICaseMap,
	}

	return c.QueryWithTextCollation(ctx, calendarHref, query)
}

// QueryByPriority finds tasks with specific priority levels.
func (c *CalDAVClient) QueryByPriority(ctx context.Context, calendarHref string, minPriority, maxPriority int) ([]CalendarObject, error) {
	query := CalendarQuery{
		Properties: []string{"calendar-data"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VTODO",
					Props: []PropFilter{
						{
							Name: "PRIORITY",
							TextMatch: &TextMatch{
								Value: fmt.Sprintf("%d", minPriority),
							},
						},
					},
				},
			},
		},
	}

	return c.QueryCalendar(ctx, calendarHref, query)
}

// QueryRecurringEvents finds all recurring events (those with RRULE, RDATE, or EXRULE).
func (c *CalDAVClient) QueryRecurringEvents(ctx context.Context, calendarHref string) ([]CalendarObject, error) {
	query := AdvancedCalendarQuery{
		Properties: []string{"calendar-data"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props: []PropFilter{
						{Name: "RRULE"},
						{Name: "RDATE"},
						{Name: "EXRULE"},
					},
				},
			},
		},
		LogicalOperator: OperatorOR,
	}

	return c.QueryWithTextCollation(ctx, calendarHref, query)
}

// QueryByTimeRange finds events within a specific time range with timezone awareness.
func (c *CalDAVClient) QueryByTimeRange(ctx context.Context, calendarHref string, start, end time.Time, timezone string) ([]CalendarObject, error) {
	query := CalendarQuery{
		Properties: []string{"calendar-data", "getetag"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					TimeRange: &TimeRange{
						Start: start,
						End:   end,
					},
				},
			},
		},
	}

	objects, err := c.QueryCalendar(ctx, calendarHref, query)
	if err != nil {
		return nil, err
	}

	if timezone != "" && timezone != "UTC" {
		return filterByTimezone(objects, timezone), nil
	}

	return objects, nil
}

func buildAdvancedQueryXML(query AdvancedCalendarQuery) (string, error) {
	var xmlBuilder strings.Builder
	xmlBuilder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	xmlBuilder.WriteString(`<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)
	xmlBuilder.WriteString(`<D:prop>`)

	for _, prop := range query.Properties {
		if prop == "calendar-data" {
			xmlBuilder.WriteString(`<C:calendar-data/>`)
		} else {
			xmlBuilder.WriteString(fmt.Sprintf(`<D:%s/>`, prop))
		}
	}

	xmlBuilder.WriteString(`</D:prop>`)
	xmlBuilder.WriteString(`<C:filter>`)

	buildAdvancedCompFilterXML(&xmlBuilder, query.Filter, query)

	xmlBuilder.WriteString(`</C:filter>`)
	xmlBuilder.WriteString(`</C:calendar-query>`)

	return xmlBuilder.String(), nil
}

func buildAdvancedCompFilterXML(builder *strings.Builder, cf Filter, query AdvancedCalendarQuery) {
	fmt.Fprintf(builder, `<C:comp-filter name="%s">`, cf.Component)

	if cf.TimeRange != nil {
		fmt.Fprintf(builder, `<C:time-range start="%s" end="%s"/>`,
			cf.TimeRange.Start.Format("20060102T150405Z"),
			cf.TimeRange.End.Format("20060102T150405Z"))
	}

	for _, propFilter := range cf.Props {
		buildAdvancedPropFilterXML(builder, propFilter, query)
	}

	for _, compFilter := range cf.CompFilters {
		buildAdvancedCompFilterXML(builder, compFilter, query)
	}

	builder.WriteString(`</C:comp-filter>`)
}

func buildAdvancedPropFilterXML(builder *strings.Builder, pf PropFilter, query AdvancedCalendarQuery) {
	fmt.Fprintf(builder, `<C:prop-filter name="%s">`, pf.Name)

	if pf.TextMatch != nil {
		collation := pf.TextMatch.Collation
		if collation == "" && query.TextCollation != "" {
			collation = string(query.TextCollation)
		}
		if collation == "" {
			collation = string(CollationASCIICaseMap)
		}

		fmt.Fprintf(builder, `<C:text-match collation="%s">%s</C:text-match>`,
			collation, pf.TextMatch.Value)
	}

	for _, paramFilter := range query.ParameterFilters {
		if paramFilter.TextMatch != "" {
			fmt.Fprintf(builder, `<C:param-filter name="%s">`, paramFilter.ParameterName)
			if paramFilter.ParameterValue != "" {
				fmt.Fprintf(builder, `<C:text-match collation="%s">%s</C:text-match>`,
					paramFilter.Collation, paramFilter.ParameterValue)
			}
			builder.WriteString(`</C:param-filter>`)
		}
	}

	builder.WriteString(`</C:prop-filter>`)
}

func buildComplexQuery(filter ComplexFilter) AdvancedCalendarQuery {
	var propFilters []PropFilter

	for _, condition := range filter.Conditions {
		pf := PropFilter{
			Name: condition.PropertyName,
		}
		if condition.PropertyValue != "" {
			collation := string(CollationASCIICaseMap)
			if condition.Collation != "" {
				collation = string(condition.Collation)
			}
			pf.TextMatch = &TextMatch{
				Value:     condition.PropertyValue,
				Collation: collation,
			}
		}
		propFilters = append(propFilters, pf)
	}

	return AdvancedCalendarQuery{
		Properties: []string{"calendar-data"},
		Filter: Filter{
			Component: "VCALENDAR",
			CompFilters: []Filter{
				{
					Component: "VEVENT",
					Props:     propFilters,
				},
			},
		},
		LogicalOperator: filter.Operator,
	}
}

func filterByTimezone(objects []CalendarObject, timezone string) []CalendarObject {
	var filtered []CalendarObject
	for _, obj := range objects {
		if strings.Contains(obj.CalendarData, "TZID="+timezone) ||
			strings.Contains(obj.CalendarData, "TZID:"+timezone) {
			filtered = append(filtered, obj)
		}
	}
	return filtered
}
