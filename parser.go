package caldav

import (
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"time"
)

type xmlMultiStatus struct {
	XMLName   xml.Name      `xml:"multistatus"`
	Responses []xmlResponse `xml:"response"`
}

type xmlResponse struct {
	Href      string        `xml:"href"`
	Propstats []xmlPropstat `xml:"propstat"`
	Status    string        `xml:"status,omitempty"`
}

type xmlPropstat struct {
	Status string      `xml:"status"`
	Prop   xmlPropData `xml:"prop"`
}

type xmlPropData struct {
	DisplayName                   string                `xml:"displayname,omitempty"`
	ResourceType                  xmlResourceType       `xml:"resourcetype,omitempty"`
	CalendarDescription           string                `xml:"calendar-description,omitempty"`
	CalendarColor                 string                `xml:"calendar-color,omitempty"`
	CalendarOrder                 string                `xml:"calendar-order,omitempty"`
	GetCTag                       string                `xml:"getctag,omitempty"`
	GetETag                       string                `xml:"getetag,omitempty"`
	CalendarData                  string                `xml:"calendar-data,omitempty"`
	GetContentType                string                `xml:"getcontenttype,omitempty"`
	CurrentUserPrincipal          xmlHref               `xml:"current-user-principal,omitempty"`
	CalendarHomeSet               xmlHref               `xml:"calendar-home-set,omitempty"`
	Owner                         xmlHref               `xml:"owner,omitempty"`
	SupportedCalendarComponentSet xmlComponentSet       `xml:"supported-calendar-component-set,omitempty"`
	CalendarTimeZone              string                `xml:"calendar-timezone,omitempty"`
	MaxResourceSize               string                `xml:"max-resource-size,omitempty"`
	MinDateTime                   string                `xml:"min-date-time,omitempty"`
	MaxDateTime                   string                `xml:"max-date-time,omitempty"`
	MaxInstances                  string                `xml:"max-instances,omitempty"`
	MaxAttendeesPerInstance       string                `xml:"max-attendees-per-instance,omitempty"`
	CurrentUserPrivilegeSet       xmlPrivilegeSet       `xml:"current-user-privilege-set,omitempty"`
	Source                        xmlHref               `xml:"source,omitempty"`
	SupportedReportSet            xmlSupportedReportSet `xml:"supported-report-set,omitempty"`
	QuotaUsedBytes                string                `xml:"quota-used-bytes,omitempty"`
	QuotaAvailableBytes           string                `xml:"quota-available-bytes,omitempty"`
	GetContentLength              string                `xml:"getcontentlength,omitempty"`
	CreationDate                  string                `xml:"creationdate,omitempty"`
	GetLastModified               string                `xml:"getlastmodified,omitempty"`
}

type xmlResourceType struct {
	Collection *struct{} `xml:"collection,omitempty"`
	Calendar   *struct{} `xml:"calendar,omitempty"`
	Principal  *struct{} `xml:"principal,omitempty"`
}

type xmlHref struct {
	Href string `xml:"href,omitempty"`
}

type xmlComponentSet struct {
	Comps []xmlComp `xml:"comp"`
}

type xmlComp struct {
	Name string `xml:"name,attr"`
}

type xmlPrivilegeSet struct {
	Privileges []xmlPrivilege `xml:"privilege"`
}

type xmlPrivilege struct {
	Read                        *struct{} `xml:"read,omitempty"`
	Write                       *struct{} `xml:"write,omitempty"`
	WriteProperties             *struct{} `xml:"write-properties,omitempty"`
	WriteContent                *struct{} `xml:"write-content,omitempty"`
	ReadCurrentUserPrivilegeSet *struct{} `xml:"read-current-user-privilege-set,omitempty"`
	ReadACL                     *struct{} `xml:"read-acl,omitempty"`
	WriteACL                    *struct{} `xml:"write-acl,omitempty"`
	All                         *struct{} `xml:"all,omitempty"`
	CalendarAccess              *struct{} `xml:"calendar-access,omitempty"`
	ReadFreeBusy                *struct{} `xml:"read-free-busy,omitempty"`
	ScheduleInbox               *struct{} `xml:"schedule-inbox,omitempty"`
	ScheduleOutbox              *struct{} `xml:"schedule-outbox,omitempty"`
	ScheduleSend                *struct{} `xml:"schedule-send,omitempty"`
	ScheduleDeliver             *struct{} `xml:"schedule-deliver,omitempty"`
}

type xmlSupportedReportSet struct {
	SupportedReports []xmlSupportedReport `xml:"supported-report"`
}

type xmlSupportedReport struct {
	Report xmlReport `xml:"report"`
}

type xmlReport struct {
	CalendarMultiget *struct{} `xml:"calendar-multiget,omitempty"`
	CalendarQuery    *struct{} `xml:"calendar-query,omitempty"`
	FreeBusyQuery    *struct{} `xml:"free-busy-query,omitempty"`
	SyncCollection   *struct{} `xml:"sync-collection,omitempty"`
}

func parseMultiStatusResponse(body io.Reader) (*MultiStatusResponse, error) {
	var ms xmlMultiStatus
	decoder := xml.NewDecoder(body)

	if err := decoder.Decode(&ms); err != nil {
		return nil, wrapErrorWithType("parse.multistatus", ErrorTypeInvalidResponse, err)
	}

	result := &MultiStatusResponse{
		Responses: make([]Response, 0, len(ms.Responses)),
	}

	for _, xmlResp := range ms.Responses {
		resp := Response{
			Href:     xmlResp.Href,
			Status:   xmlResp.Status,
			Propstat: make([]Propstat, 0, len(xmlResp.Propstats)),
		}

		for _, xmlPS := range xmlResp.Propstats {
			ps := Propstat{
				Status: parseStatusCode(xmlPS.Status),
				Prop:   convertPropData(xmlPS.Prop),
			}
			resp.Propstat = append(resp.Propstat, ps)
		}

		result.Responses = append(result.Responses, resp)
	}

	return result, nil
}

func parseStatusCode(status string) int {
	parts := strings.Fields(status)
	if len(parts) < 2 {
		return 0
	}

	code, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}

	return code
}

func parseDateTime(dateStr string) (time.Time, error) {
	return ParseCalDAVTime(dateStr)
}

func parseNumericFields(xmlProp xmlPropData, prop *PropstatProp) {
	if xmlProp.MaxResourceSize != "" {
		if size, err := strconv.ParseInt(xmlProp.MaxResourceSize, 10, 64); err == nil {
			prop.MaxResourceSize = size
		}
	}
	if xmlProp.MaxInstances != "" {
		if instances, err := strconv.Atoi(xmlProp.MaxInstances); err == nil {
			prop.MaxInstances = instances
		}
	}
	if xmlProp.MaxAttendeesPerInstance != "" {
		if attendees, err := strconv.Atoi(xmlProp.MaxAttendeesPerInstance); err == nil {
			prop.MaxAttendeesPerInstance = attendees
		}
	}
	if xmlProp.QuotaUsedBytes != "" {
		if quota, err := strconv.ParseInt(xmlProp.QuotaUsedBytes, 10, 64); err == nil {
			prop.QuotaUsedBytes = quota
		}
	}
	if xmlProp.QuotaAvailableBytes != "" {
		if quota, err := strconv.ParseInt(xmlProp.QuotaAvailableBytes, 10, 64); err == nil {
			prop.QuotaAvailableBytes = quota
		}
	}
	if xmlProp.GetContentLength != "" {
		if length, err := strconv.ParseInt(xmlProp.GetContentLength, 10, 64); err == nil {
			prop.ContentLength = length
		}
	}
}

func parseResourceTypes(xmlProp xmlPropData) []string {
	var resourceTypes []string
	if xmlProp.ResourceType.Collection != nil {
		resourceTypes = append(resourceTypes, "collection")
	}
	if xmlProp.ResourceType.Calendar != nil {
		resourceTypes = append(resourceTypes, "calendar")
	}
	if xmlProp.ResourceType.Principal != nil {
		resourceTypes = append(resourceTypes, "principal")
	}
	return resourceTypes
}

func parseSupportedComponents(xmlProp xmlPropData) []string {
	var components []string
	for _, comp := range xmlProp.SupportedCalendarComponentSet.Comps {
		components = append(components, comp.Name)
	}
	return components
}

func parsePrivilegeSet(xmlProp xmlPropData) []string {
	var privileges []string
	for _, priv := range xmlProp.CurrentUserPrivilegeSet.Privileges {
		privileges = append(privileges, extractPrivilegeNames(priv)...)
	}
	return privileges
}

func extractPrivilegeNames(priv xmlPrivilege) []string {
	var names []string

	type privilegeCheck struct {
		field *struct{}
		name  string
	}

	checks := []privilegeCheck{
		{priv.Read, "read"},
		{priv.Write, "write"},
		{priv.WriteProperties, "write-properties"},
		{priv.WriteContent, "write-content"},
		{priv.ReadCurrentUserPrivilegeSet, "read-current-user-privilege-set"},
		{priv.ReadACL, "read-acl"},
		{priv.WriteACL, "write-acl"},
		{priv.All, "all"},
		{priv.CalendarAccess, "calendar-access"},
		{priv.ReadFreeBusy, "read-free-busy"},
		{priv.ScheduleInbox, "schedule-inbox"},
		{priv.ScheduleOutbox, "schedule-outbox"},
		{priv.ScheduleSend, "schedule-send"},
		{priv.ScheduleDeliver, "schedule-deliver"},
	}

	for _, check := range checks {
		if check.field != nil {
			names = append(names, check.name)
		}
	}

	return names
}

func parseSupportedReports(xmlProp xmlPropData) []string {
	var reports []string
	for _, report := range xmlProp.SupportedReportSet.SupportedReports {
		if report.Report.CalendarMultiget != nil {
			reports = append(reports, "calendar-multiget")
		}
		if report.Report.CalendarQuery != nil {
			reports = append(reports, "calendar-query")
		}
		if report.Report.FreeBusyQuery != nil {
			reports = append(reports, "free-busy-query")
		}
		if report.Report.SyncCollection != nil {
			reports = append(reports, "sync-collection")
		}
	}
	return reports
}

func convertPropData(xmlProp xmlPropData) PropstatProp {
	prop := PropstatProp{
		DisplayName:          xmlProp.DisplayName,
		CalendarDescription:  xmlProp.CalendarDescription,
		CalendarColor:        xmlProp.CalendarColor,
		CTag:                 xmlProp.GetCTag,
		ETag:                 xmlProp.GetETag,
		CalendarData:         xmlProp.CalendarData,
		CurrentUserPrincipal: xmlProp.CurrentUserPrincipal.Href,
		CalendarHomeSet:      xmlProp.CalendarHomeSet.Href,
		Owner:                xmlProp.Owner.Href,
		CalendarTimeZone:     xmlProp.CalendarTimeZone,
		MinDateTime:          xmlProp.MinDateTime,
		MaxDateTime:          xmlProp.MaxDateTime,
		Source:               xmlProp.Source.Href,
		ContentType:          xmlProp.GetContentType,
		CreationDate:         xmlProp.CreationDate,
		LastModified:         xmlProp.GetLastModified,
	}

	parseNumericFields(xmlProp, &prop)
	prop.ResourceType = parseResourceTypes(xmlProp)
	prop.SupportedCalendarComponentSet = parseSupportedComponents(xmlProp)
	prop.CurrentUserPrivilegeSet = parsePrivilegeSet(xmlProp)
	prop.SupportedReports = parseSupportedReports(xmlProp)

	return prop
}

func extractCalendarsFromResponse(resp *MultiStatusResponse) []Calendar {
	calendars := make([]Calendar, 0)

	for _, r := range resp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 || ps.Status == 0 {
				isCalendar := false
				for _, rt := range ps.Prop.ResourceType {
					if rt == "calendar" {
						isCalendar = true
						break
					}
				}

				if isCalendar {
					cal := Calendar{
						Href:                    r.Href,
						Name:                    extractNameFromHref(r.Href),
						DisplayName:             ps.Prop.DisplayName,
						Description:             ps.Prop.CalendarDescription,
						Color:                   ps.Prop.CalendarColor,
						ResourceType:            ps.Prop.ResourceType,
						SupportedComponents:     ps.Prop.SupportedCalendarComponentSet,
						CTag:                    ps.Prop.CTag,
						ETag:                    ps.Prop.ETag,
						CalendarTimeZone:        ps.Prop.CalendarTimeZone,
						MaxResourceSize:         ps.Prop.MaxResourceSize,
						MaxInstances:            ps.Prop.MaxInstances,
						MaxAttendeesPerInstance: ps.Prop.MaxAttendeesPerInstance,
						CurrentUserPrivilegeSet: ps.Prop.CurrentUserPrivilegeSet,
						Source:                  ps.Prop.Source,
						SupportedReports:        ps.Prop.SupportedReports,
						Quota: CalendarQuota{
							QuotaUsedBytes:      ps.Prop.QuotaUsedBytes,
							QuotaAvailableBytes: ps.Prop.QuotaAvailableBytes,
						},
					}

					// Parse min and max date times if present
					if ps.Prop.MinDateTime != "" {
						if minDt, err := parseDateTime(ps.Prop.MinDateTime); err == nil {
							cal.MinDateTime = &minDt
						}
					}
					if ps.Prop.MaxDateTime != "" {
						if maxDt, err := parseDateTime(ps.Prop.MaxDateTime); err == nil {
							cal.MaxDateTime = &maxDt
						}
					}

					calendars = append(calendars, cal)
				}
			}
		}
	}

	return calendars
}

// extractCalendarObjectsFromResponse extracts calendar objects from a multi-status response.
// This function is exported for use in tests and benchmarks.
func extractCalendarObjectsFromResponse(resp *MultiStatusResponse) []CalendarObject {
	return extractCalendarObjectsFromResponseWithOptions(resp, false)
}

func extractCalendarObjectsFromResponseWithOptions(resp *MultiStatusResponse, autoParsing bool) []CalendarObject {
	objects := make([]CalendarObject, 0)

	for _, r := range resp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 {
				if ps.Prop.CalendarData != "" {
					obj := CalendarObject{
						Href:         r.Href,
						ETag:         ps.Prop.ETag,
						CalendarData: ps.Prop.CalendarData,
					}

					parseCalendarData(&obj, ps.Prop.CalendarData)

					if autoParsing && ps.Prop.CalendarData != "" {
						parsedData, err := ParseICalendar(ps.Prop.CalendarData)
						if err != nil {
							obj.ParseError = err
						} else {
							obj.ParsedData = parsedData
						}
					}

					objects = append(objects, obj)
				} else if ps.Prop.ETag != "" {
					obj := CalendarObject{
						Href: r.Href,
						ETag: ps.Prop.ETag,
					}
					objects = append(objects, obj)
				}
			}
		}
	}

	return objects
}

func parseCalendarData(obj *CalendarObject, data string) {
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		parseCalendarLine(obj, line)
	}
}

func parseCalendarLine(obj *CalendarObject, line string) {
	simpleFields := map[string]*string{
		"UID:":         &obj.UID,
		"SUMMARY:":     &obj.Summary,
		"DESCRIPTION:": &obj.Description,
		"LOCATION:":    &obj.Location,
		"STATUS:":      &obj.Status,
		"ORGANIZER:":   &obj.Organizer,
		"RRULE:":       &obj.RecurrenceRule,
		"CLASS:":       &obj.Class,
		"TRANSP:":      &obj.Transparency,
	}

	for prefix, field := range simpleFields {
		if strings.HasPrefix(line, prefix) {
			*field = strings.TrimPrefix(line, prefix)
			return
		}
	}

	parseCalendarTimeFields(obj, line)
	parseCalendarAttendee(obj, line)
}

func parseCalendarTimeFields(obj *CalendarObject, line string) {
	timeFields := []struct {
		prefixes []string
		field    **time.Time
	}{
		{[]string{"DTSTART:", "DTSTART;"}, &obj.StartTime},
		{[]string{"DTEND:", "DTEND;"}, &obj.EndTime},
		{[]string{"CREATED:"}, &obj.Created},
		{[]string{"LAST-MODIFIED:"}, &obj.LastModified},
	}

	for _, tf := range timeFields {
		for _, prefix := range tf.prefixes {
			if strings.HasPrefix(line, prefix) {
				if t := parseICalTime(line); t != nil {
					*tf.field = t
				}
				return
			}
		}
	}
}

func parseCalendarAttendee(obj *CalendarObject, line string) {
	if after, ok := strings.CutPrefix(line, "ATTENDEE:"); ok {
		attendee := after
		if after, ok := strings.CutPrefix(attendee, "mailto:"); ok {
			attendee = after
		}
		obj.Attendees = append(obj.Attendees, attendee)
	}
}

func parseICalTime(line string) *time.Time {
	return ParseICalPropertyTime(line)
}

func extractNameFromHref(href string) string {
	parts := strings.Split(strings.TrimSuffix(href, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return href
}

func extractPrincipalFromResponse(resp *MultiStatusResponse) string {
	for _, r := range resp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 && ps.Prop.CurrentUserPrincipal != "" {
				return ps.Prop.CurrentUserPrincipal
			}
		}
	}
	return ""
}

func extractCalendarHomeSetFromResponse(resp *MultiStatusResponse) string {
	for _, r := range resp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 && ps.Prop.CalendarHomeSet != "" {
				return ps.Prop.CalendarHomeSet
			}
		}
	}
	return ""
}
