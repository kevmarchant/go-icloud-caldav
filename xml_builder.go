package caldav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"time"
)

func buildPropfindXML(props []string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	buf.WriteString(`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/" xmlns:A="http://apple.com/ns/ical/">`)
	buf.WriteString(`<D:prop>`)

	for _, prop := range props {
		switch prop {
		case "displayname":
			buf.WriteString(`<D:displayname/>`)
		case "resourcetype":
			buf.WriteString(`<D:resourcetype/>`)
		case "current-user-principal":
			buf.WriteString(`<D:current-user-principal/>`)
		case "owner":
			buf.WriteString(`<D:owner/>`)
		case "calendar-home-set":
			buf.WriteString(`<C:calendar-home-set/>`)
		case "calendar-description":
			buf.WriteString(`<C:calendar-description/>`)
		case "calendar-color":
			buf.WriteString(`<A:calendar-color/>`)
		case "calendar-order":
			buf.WriteString(`<A:calendar-order/>`)
		case "supported-calendar-component-set":
			buf.WriteString(`<C:supported-calendar-component-set/>`)
		case "getctag":
			buf.WriteString(`<CS:getctag/>`)
		case "getetag":
			buf.WriteString(`<D:getetag/>`)
		case "calendar-data":
			buf.WriteString(`<C:calendar-data/>`)
		}
	}

	buf.WriteString(`</D:prop>`)
	buf.WriteString(`</D:propfind>`)

	return buf.Bytes(), nil
}

func buildCalendarQueryXML(query CalendarQuery) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	buf.WriteString(`<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	buf.WriteString(`<D:prop>`)
	for _, prop := range query.Properties {
		switch prop {
		case "getetag":
			buf.WriteString(`<D:getetag/>`)
		case "calendar-data":
			buf.WriteString(`<C:calendar-data/>`)
		}
	}
	buf.WriteString(`</D:prop>`)

	if query.Filter.Component != "" || query.TimeRange != nil {
		buf.WriteString(`<C:filter>`)
		buf.WriteString(`<C:comp-filter name="VCALENDAR">`)

		if query.Filter.Component != "" {
			buf.WriteString(fmt.Sprintf(`<C:comp-filter name="%s">`, query.Filter.Component))

			for _, propFilter := range query.Filter.Props {
				buf.WriteString(fmt.Sprintf(`<C:prop-filter name="%s">`, propFilter.Name))

				if propFilter.TextMatch != nil {
					negateAttr := ""
					if propFilter.TextMatch.NegateCondition {
						negateAttr = ` negate-condition="yes"`
					}
					collationAttr := ""
					if propFilter.TextMatch.Collation != "" {
						collationAttr = fmt.Sprintf(` collation="%s"`, propFilter.TextMatch.Collation)
					}
					buf.WriteString(fmt.Sprintf(`<C:text-match%s%s>%s</C:text-match>`,
						collationAttr, negateAttr, xmlEscape(propFilter.TextMatch.Value)))
				}

				if propFilter.TimeRange != nil {
					writeTimeRange(&buf, propFilter.TimeRange)
				}

				buf.WriteString(`</C:prop-filter>`)
			}

			if query.Filter.TimeRange != nil {
				writeTimeRange(&buf, query.Filter.TimeRange)
			}

			buf.WriteString(`</C:comp-filter>`)
		} else if query.TimeRange != nil {
			buf.WriteString(`<C:comp-filter name="VEVENT">`)
			writeTimeRange(&buf, query.TimeRange)
			buf.WriteString(`</C:comp-filter>`)
		}

		buf.WriteString(`</C:comp-filter>`)
		buf.WriteString(`</C:filter>`)
	}

	buf.WriteString(`</C:calendar-query>`)

	return buf.Bytes(), nil
}

func writeTimeRange(buf *bytes.Buffer, tr *TimeRange) {
	startStr := formatTimeForCalDAV(tr.Start)
	endStr := formatTimeForCalDAV(tr.End)
	fmt.Fprintf(buf, `<C:time-range start="%s" end="%s"/>`, startStr, endStr)
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func formatTimeForCalDAV(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}
