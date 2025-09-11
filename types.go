// Package caldav provides a CalDAV client implementation specifically designed for iCloud compatibility.
// It handles proper XML namespace generation and multi-status response parsing required by iCloud's CalDAV implementation.
package caldav

import (
	"time"
)

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
}

type CalendarQuery struct {
	Properties []string
	Filter     Filter
	TimeRange  *TimeRange
}

type Filter struct {
	Component string
	Props     []PropFilter
	TimeRange *TimeRange
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
