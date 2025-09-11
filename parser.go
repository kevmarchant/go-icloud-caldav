package caldav

import (
	"encoding/xml"
	"fmt"
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
	DisplayName                   string          `xml:"displayname,omitempty"`
	ResourceType                  xmlResourceType `xml:"resourcetype,omitempty"`
	CalendarDescription           string          `xml:"calendar-description,omitempty"`
	CalendarColor                 string          `xml:"calendar-color,omitempty"`
	CalendarOrder                 string          `xml:"calendar-order,omitempty"`
	GetCTag                       string          `xml:"getctag,omitempty"`
	GetETag                       string          `xml:"getetag,omitempty"`
	CalendarData                  string          `xml:"calendar-data,omitempty"`
	CurrentUserPrincipal          xmlHref         `xml:"current-user-principal,omitempty"`
	CalendarHomeSet               xmlHref         `xml:"calendar-home-set,omitempty"`
	Owner                         xmlHref         `xml:"owner,omitempty"`
	SupportedCalendarComponentSet xmlComponentSet `xml:"supported-calendar-component-set,omitempty"`
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

func parseMultiStatusResponse(body io.Reader) (*MultiStatusResponse, error) {
	var ms xmlMultiStatus
	decoder := xml.NewDecoder(body)

	if err := decoder.Decode(&ms); err != nil {
		return nil, fmt.Errorf("decoding multistatus response: %w", err)
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
	}

	if xmlProp.ResourceType.Collection != nil {
		prop.ResourceType = append(prop.ResourceType, "collection")
	}
	if xmlProp.ResourceType.Calendar != nil {
		prop.ResourceType = append(prop.ResourceType, "calendar")
	}
	if xmlProp.ResourceType.Principal != nil {
		prop.ResourceType = append(prop.ResourceType, "principal")
	}

	for _, comp := range xmlProp.SupportedCalendarComponentSet.Comps {
		prop.SupportedCalendarComponentSet = append(prop.SupportedCalendarComponentSet, comp.Name)
	}

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
						Href:                r.Href,
						Name:                extractNameFromHref(r.Href),
						DisplayName:         ps.Prop.DisplayName,
						Description:         ps.Prop.CalendarDescription,
						Color:               ps.Prop.CalendarColor,
						ResourceType:        ps.Prop.ResourceType,
						SupportedComponents: ps.Prop.SupportedCalendarComponentSet,
						CTag:                ps.Prop.CTag,
						ETag:                ps.Prop.ETag,
					}
					calendars = append(calendars, cal)
				}
			}
		}
	}

	return calendars
}

func extractCalendarObjectsFromResponse(resp *MultiStatusResponse) []CalendarObject {
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

		if strings.HasPrefix(line, "UID:") {
			obj.UID = strings.TrimPrefix(line, "UID:")
		} else if strings.HasPrefix(line, "SUMMARY:") {
			obj.Summary = strings.TrimPrefix(line, "SUMMARY:")
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			obj.Description = strings.TrimPrefix(line, "DESCRIPTION:")
		} else if strings.HasPrefix(line, "LOCATION:") {
			obj.Location = strings.TrimPrefix(line, "LOCATION:")
		} else if strings.HasPrefix(line, "STATUS:") {
			obj.Status = strings.TrimPrefix(line, "STATUS:")
		} else if strings.HasPrefix(line, "ORGANIZER:") {
			obj.Organizer = strings.TrimPrefix(line, "ORGANIZER:")
		} else if strings.HasPrefix(line, "DTSTART:") || strings.HasPrefix(line, "DTSTART;") {
			if t := parseICalTime(line); t != nil {
				obj.StartTime = t
			}
		} else if strings.HasPrefix(line, "DTEND:") || strings.HasPrefix(line, "DTEND;") {
			if t := parseICalTime(line); t != nil {
				obj.EndTime = t
			}
		} else if strings.HasPrefix(line, "CREATED:") {
			if t := parseICalTime(line); t != nil {
				obj.Created = t
			}
		} else if strings.HasPrefix(line, "LAST-MODIFIED:") {
			if t := parseICalTime(line); t != nil {
				obj.LastModified = t
			}
		} else if after, ok := strings.CutPrefix(line, "ATTENDEE:"); ok {
			attendee := after
			if after, ok := strings.CutPrefix(attendee, "mailto:"); ok {
				attendee = after
			}
			obj.Attendees = append(obj.Attendees, attendee)
		}
	}
}

func parseICalTime(line string) *time.Time {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	timeStr := parts[1]

	formats := []string{
		"20060102T150405Z",
		"20060102T150405",
		"20060102",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return &t
		}
	}

	return nil
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
