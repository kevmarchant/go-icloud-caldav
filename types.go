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
	RecurrenceDates  []time.Time
	ExceptionDates   []time.Time
	ExceptionRule    string
	RelatedTo        []RelatedEvent
	Attachments      []Attachment
	Contacts         []string
	Comments         []string
	RequestStatus    []RequestStatus
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
	RelatedTo        []RelatedEvent
	Attachments      []Attachment
	Contacts         []string
	Comments         []string
	RequestStatus    []RequestStatus
	Created          *time.Time
	LastModified     *time.Time
	Sequence         int
	Class            string
	URL              string
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
	DTStart          *time.Time
	TZOffsetFrom     string
	TZOffsetTo       string
	TZName           string
	RecurrenceRule   string
	RecurrenceDates  []time.Time
	ExceptionDates   []time.Time
	Comment          []string
	CustomProperties map[string]string
}

// TimeZoneTransition represents a timezone transition point.
type TimeZoneTransition struct {
	DateTime     time.Time
	OffsetFrom   time.Duration
	OffsetTo     time.Duration
	Abbreviation string
	IsDST        bool
}

// TimeZoneInfo provides timezone calculation and DST transition information.
type TimeZoneInfo struct {
	TZID        string
	Location    *time.Location
	Transitions []TimeZoneTransition
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

// RelatedEvent represents event relationship information from RELATED-TO property.
type RelatedEvent struct {
	UID          string
	RelationType string
}

// Attachment represents file attachment information from ATTACH property.
type Attachment struct {
	URI          string
	Encoding     string
	Value        string
	FormatType   string
	Size         int
	Filename     string
	CustomParams map[string]string
}

// RequestStatus represents meeting request status information from REQUEST-STATUS property.
type RequestStatus struct {
	Code        string
	Description string
	ExtraData   string
}

// CalendarQuota represents quota information for a calendar.
type CalendarQuota struct {
	QuotaUsedBytes      int64
	QuotaAvailableBytes int64
}

// Principal represents a CalDAV principal (user, group, or resource).
type Principal struct {
	Href        string
	DisplayName string
	Email       string
	Type        string // "user", "group", "resource"
}

// ACE represents an Access Control Entry.
type ACE struct {
	Principal Principal
	Grant     []string // List of granted privileges
	Deny      []string // List of denied privileges
	Protected bool     // Whether this ACE is protected from deletion
	Inherited string   // Inherited from which resource (if any)
}

// ACL represents an Access Control List.
type ACL struct {
	ACEs []ACE
}

// PrivilegeSet represents a set of privileges.
type PrivilegeSet struct {
	Read                        bool
	Write                       bool
	WriteProperties             bool
	WriteContent                bool
	ReadCurrentUserPrivilegeSet bool
	ReadACL                     bool
	WriteACL                    bool
	All                         bool
	CalendarAccess              bool
	ReadFreeBusy                bool
	ScheduleInbox               bool
	ScheduleOutbox              bool
	ScheduleSend                bool
	ScheduleDeliver             bool
}

// Calendar represents a CalDAV calendar collection with its properties.
type Calendar struct {
	Name                    string
	DisplayName             string
	Href                    string
	Description             string
	Color                   string
	SupportedComponents     []string
	ResourceType            []string
	CTag                    string
	ETag                    string
	CalendarTimeZone        string
	MaxResourceSize         int64
	MinDateTime             *time.Time
	MaxDateTime             *time.Time
	MaxInstances            int
	MaxAttendeesPerInstance int
	CurrentUserPrivilegeSet []string
	Source                  string
	SupportedReports        []string
	Quota                   CalendarQuota
	ACL                     ACL
}

type CalendarObject struct {
	Href             string
	ETag             string
	CalendarData     string
	UID              string
	Summary          string
	Description      string
	Location         string
	StartTime        *time.Time
	EndTime          *time.Time
	Organizer        string
	Attendees        []string
	Status           string
	Created          *time.Time
	LastModified     *time.Time
	RecurrenceRule   string
	RecurrenceID     *time.Time
	ExceptionDates   []time.Time
	RecurrenceDates  []time.Time
	Categories       []string
	Class            string
	Priority         int
	Transparency     string
	URL              string
	CustomProperties map[string]string
	ParsedData       *ParsedCalendarData
	ParseError       error
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
	CalendarTimeZone              string
	MaxResourceSize               int64
	MinDateTime                   string
	MaxDateTime                   string
	MaxInstances                  int
	MaxAttendeesPerInstance       int
	CurrentUserPrivilegeSet       []string
	Source                        string
	SupportedReports              []string
	QuotaUsedBytes                int64
	QuotaAvailableBytes           int64
	ContentType                   string
	ContentLength                 int64
	CreationDate                  string
	LastModified                  string
}

// CalendarHomeSet represents the calendar home collection URL.
type CalendarHomeSet struct {
	Href string // Href is the URL to the calendar home collection
}

// CurrentUserPrincipal represents the current authenticated user's principal URL.
type CurrentUserPrincipal struct {
	Href string // Href is the URL to the user's principal
}
