// Package caldav provides a CalDAV client implementation specifically designed for iCloud compatibility.
// It handles proper XML namespace generation and multi-status response parsing required by iCloud's CalDAV implementation.
package caldav

import (
	"time"
)

// ParsedCalendarData contains structured calendar data parsed from iCalendar format.
// This provides easier access to calendar components without manual parsing.
type ParsedCalendarData struct {
	Version          string
	ProdID           string
	CalScale         string
	Method           string
	Events           []ParsedEvent
	Todos            []ParsedTodo
	Journals         []ParsedJournal
	FreeBusy         []ParsedFreeBusy
	TimeZones        []ParsedTimeZone
	Alarms           []ParsedAlarm
	CustomProperties map[string]string
}

// ParsedEvent represents a parsed VEVENT component.
type ParsedEvent struct {
	UID              string
	DTStamp          *time.Time
	DTStart          *time.Time
	DTEnd            *time.Time
	Duration         string
	Summary          string
	Description      string
	Location         string
	Status           string
	Transparency     string
	Categories       []string
	Organizer        ParsedOrganizer
	Attendees        []ParsedAttendee
	RecurrenceID     *time.Time
	RecurrenceRule   string
	ExceptionDates   []time.Time
	Created          *time.Time
	LastModified     *time.Time
	Sequence         int
	Priority         int
	Class            string
	URL              string
	GeoLocation      *GeoLocation
	Alarms           []ParsedAlarm
	CustomProperties map[string]string
}

// ParsedTodo represents a parsed VTODO component.
type ParsedTodo struct {
	UID              string
	DTStamp          *time.Time
	DTStart          *time.Time
	Due              *time.Time
	Completed        *time.Time
	Summary          string
	Description      string
	Status           string
	PercentComplete  int
	Priority         int
	Categories       []string
	Created          *time.Time
	LastModified     *time.Time
	Sequence         int
	Class            string
	URL              string
	CustomProperties map[string]string
}

// ParsedJournal represents a parsed VJOURNAL component.
type ParsedJournal struct {
	UID              string
	DTStamp          *time.Time
	DTStart          *time.Time
	Summary          string
	Description      string
	Status           string
	Categories       []string
	Created          *time.Time
	LastModified     *time.Time
	Sequence         int
	Class            string
	CustomProperties map[string]string
}

// ParsedFreeBusy represents a parsed VFREEBUSY component.
type ParsedFreeBusy struct {
	UID              string
	DTStamp          *time.Time
	DTStart          *time.Time
	DTEnd            *time.Time
	Organizer        ParsedOrganizer
	Attendees        []ParsedAttendee
	FreeBusy         []FreeBusyPeriod
	CustomProperties map[string]string
}

// ParsedTimeZone represents a parsed VTIMEZONE component.
type ParsedTimeZone struct {
	TZID             string
	StandardTime     ParsedTimeZoneComponent
	DaylightTime     ParsedTimeZoneComponent
	CustomProperties map[string]string
}

// ParsedTimeZoneComponent represents standard or daylight time information.
type ParsedTimeZoneComponent struct {
	DTStart        *time.Time
	TZOffsetFrom   string
	TZOffsetTo     string
	TZName         string
	RecurrenceRule string
}

// ParsedAlarm represents a parsed VALARM component.
type ParsedAlarm struct {
	Action           string
	Trigger          string
	Duration         string
	Repeat           int
	Description      string
	Summary          string
	Attendees        []ParsedAttendee
	CustomProperties map[string]string
}

// ParsedOrganizer represents event/todo organizer information.
type ParsedOrganizer struct {
	Value        string
	CN           string
	Email        string
	Dir          string
	SentBy       string
	CustomParams map[string]string
}

// ParsedAttendee represents attendee information.
type ParsedAttendee struct {
	Value         string
	CN            string
	Email         string
	Role          string
	PartStat      string
	RSVP          bool
	CUType        string
	Member        string
	DelegatedTo   string
	DelegatedFrom string
	Dir           string
	SentBy        string
	CustomParams  map[string]string
}

// GeoLocation represents geographic coordinates.
type GeoLocation struct {
	Latitude  float64
	Longitude float64
}

// FreeBusyPeriod represents a free/busy time period.
type FreeBusyPeriod struct {
	Start  time.Time
	End    time.Time
	FBType string
}

// Calendar represents a CalDAV calendar collection with its properties.
type Calendar struct {
	Name                string
	DisplayName         string
	Href                string
	Description         string
	Color               string
	SupportedComponents []string
	ResourceType        []string
	CTag                string
	ETag                string
}

type CalendarObject struct {
	Href         string
	ETag         string
	CalendarData string
	UID          string
	Summary      string
	Description  string
	Location     string
	StartTime    *time.Time
	EndTime      *time.Time
	Organizer    string
	Attendees    []string
	Status       string
	Created      *time.Time
	LastModified *time.Time
	ParsedData   *ParsedCalendarData
	ParseError   error
}

type CalendarQuery struct {
	Properties []string
	Filter     Filter
	TimeRange  *TimeRange
}

type Filter struct {
	Component   string
	Props       []PropFilter
	TimeRange   *TimeRange
	CompFilters []Filter
}

type PropFilter struct {
	Name      string
	TextMatch *TextMatch
	TimeRange *TimeRange
}

type TextMatch struct {
	Value           string
	Collation       string
	NegateCondition bool
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type MultiStatusResponse struct {
	Responses []Response
}

type Response struct {
	Href     string
	Propstat []Propstat
	Status   string
}

type Propstat struct {
	Status int
	Prop   PropstatProp
}

type PropstatProp struct {
	CalendarData                  string
	DisplayName                   string
	CalendarDescription           string
	CalendarColor                 string
	SupportedCalendarComponentSet []string
	ResourceType                  []string
	CTag                          string
	ETag                          string
	CurrentUserPrincipal          string
	CalendarHomeSet               string
	Owner                         string
}

// CalendarHomeSet represents the calendar home collection URL.
type CalendarHomeSet struct {
	Href string // Href is the URL to the calendar home collection
}

// CurrentUserPrincipal represents the current authenticated user's principal URL.
type CurrentUserPrincipal struct {
	Href string // Href is the URL to the user's principal
}
