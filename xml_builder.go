package caldav

import (
	"bytes"
	"strings"
	"time"
)

var propElementMap = map[string]string{
	"displayname":                      `<D:displayname/>`,
	"resourcetype":                     `<D:resourcetype/>`,
	"current-user-principal":           `<D:current-user-principal/>`,
	"owner":                            `<D:owner/>`,
	"calendar-home-set":                `<C:calendar-home-set/>`,
	"calendar-description":             `<C:calendar-description/>`,
	"calendar-color":                   `<A:calendar-color/>`,
	"calendar-order":                   `<A:calendar-order/>`,
	"supported-calendar-component-set": `<C:supported-calendar-component-set/>`,
	"getctag":                          `<CS:getctag/>`,
	"getetag":                          `<D:getetag/>`,
	"calendar-data":                    `<C:calendar-data/>`,
	"calendar-timezone":                `<C:calendar-timezone/>`,
	"max-resource-size":                `<C:max-resource-size/>`,
	"min-date-time":                    `<C:min-date-time/>`,
	"max-date-time":                    `<C:max-date-time/>`,
	"max-instances":                    `<C:max-instances/>`,
	"max-attendees-per-instance":       `<C:max-attendees-per-instance/>`,
	"current-user-privilege-set":       `<D:current-user-privilege-set/>`,
	"source":                           `<D:source/>`,
	"supported-report-set":             `<D:supported-report-set/>`,
	"quota-used-bytes":                 `<D:quota-used-bytes/>`,
	"quota-available-bytes":            `<D:quota-available-bytes/>`,
	"acl":                              `<D:acl/>`,
	"supported-privilege-set":          `<D:supported-privilege-set/>`,
	"acl-restrictions":                 `<D:acl-restrictions/>`,
	"inherited-acl-set":                `<D:inherited-acl-set/>`,
	"principal-URL":                    `<D:principal-URL/>`,
	"alternate-URI-set":                `<D:alternate-URI-set/>`,
	"group-member-set":                 `<D:group-member-set/>`,
	"group-membership":                 `<D:group-membership/>`,
	"calendar-user-address-set":        `<C:calendar-user-address-set/>`,
	"schedule-inbox-URL":               `<C:schedule-inbox-URL/>`,
	"schedule-outbox-URL":              `<C:schedule-outbox-URL/>`,
}

const (
	xmlHeader     = `<?xml version="1.0" encoding="utf-8"?>`
	propfindStart = `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/" xmlns:A="http://apple.com/ns/ical/">`
	propStart     = `<D:prop>`
	propEnd       = `</D:prop>`
	propfindEnd   = `</D:propfind>`

	avgPropElementSize = 45
	baseXMLOverhead    = 200
)

func buildPropfindXML(props []string) ([]byte, error) {
	if len(props) == 0 {
		return buildEmptyPropfindXML(), nil
	}

	estimatedSize := baseXMLOverhead + len(props)*avgPropElementSize
	buf := make([]byte, 0, estimatedSize)

	buf = append(buf, xmlHeader...)
	buf = append(buf, propfindStart...)
	buf = append(buf, propStart...)

	for _, prop := range props {
		if element, exists := propElementMap[prop]; exists {
			buf = append(buf, element...)
		}
	}

	buf = append(buf, propEnd...)
	buf = append(buf, propfindEnd...)

	return buf, nil
}

func buildEmptyPropfindXML() []byte {
	return []byte(xmlHeader + propfindStart + propStart + propEnd + propfindEnd)
}

type XMLBuilder struct {
	buf    bytes.Buffer
	indent int
}

func NewXMLBuilder(initialCapacity int) *XMLBuilder {
	builder := &XMLBuilder{}
	builder.buf.Grow(initialCapacity)
	return builder
}

func (xb *XMLBuilder) WriteHeader() *XMLBuilder {
	xb.buf.WriteString(xmlHeader)
	return xb
}

func (xb *XMLBuilder) WriteStartElement(name string, attrs ...string) *XMLBuilder {
	xb.buf.WriteByte('<')
	xb.buf.WriteString(name)

	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			xb.buf.WriteString(` `)
			xb.buf.WriteString(attrs[i])
			xb.buf.WriteString(`="`)
			xb.buf.WriteString(xmlEscape(attrs[i+1]))
			xb.buf.WriteByte('"')
		}
	}

	xb.buf.WriteByte('>')
	return xb
}

func (xb *XMLBuilder) WriteEndElement(name string) *XMLBuilder {
	xb.buf.WriteString(`</`)
	xb.buf.WriteString(name)
	xb.buf.WriteByte('>')
	return xb
}

func (xb *XMLBuilder) WriteSelfClosingElement(name string, attrs ...string) *XMLBuilder {
	xb.buf.WriteByte('<')
	xb.buf.WriteString(name)

	for i := 0; i < len(attrs); i += 2 {
		if i+1 < len(attrs) {
			xb.buf.WriteString(` `)
			xb.buf.WriteString(attrs[i])
			xb.buf.WriteString(`="`)
			xb.buf.WriteString(xmlEscape(attrs[i+1]))
			xb.buf.WriteByte('"')
		}
	}

	xb.buf.WriteString(`/>`)
	return xb
}

func (xb *XMLBuilder) WriteText(text string) *XMLBuilder {
	xb.buf.WriteString(xmlEscape(text))
	return xb
}

func (xb *XMLBuilder) WriteRawString(raw string) *XMLBuilder {
	xb.buf.WriteString(raw)
	return xb
}

func (xb *XMLBuilder) Bytes() []byte {
	return xb.buf.Bytes()
}

func (xb *XMLBuilder) String() string {
	return xb.buf.String()
}

func (xb *XMLBuilder) Reset() {
	xb.buf.Reset()
	xb.indent = 0
}

var xmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&#34;",
	"'", "&#39;",
)

func xmlEscape(s string) string {
	if !needsEscaping(s) {
		return s
	}
	return xmlReplacer.Replace(s)
}

func needsEscaping(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"', '\'':
			return true
		}
	}
	return false
}

func formatTimeForCalDAV(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func calculateQueryXMLSize(query CalendarQuery) int {
	size := 500 + len(query.Properties)*30
	if query.Filter.Component != "" {
		size += 200
	}
	if query.TimeRange != nil {
		size += 100
	}
	return size
}

func needsFilter(query CalendarQuery) bool {
	return query.Filter.Component != "" || query.TimeRange != nil || len(query.Filter.CompFilters) > 0
}

func writeQueryProperties(builder *XMLBuilder, properties []string) {
	for _, prop := range properties {
		switch prop {
		case "getetag":
			builder.WriteSelfClosingElement("D:getetag")
		case "calendar-data":
			builder.WriteSelfClosingElement("C:calendar-data")
		default:
			if element, exists := propElementMap[prop]; exists {
				builder.WriteRawString(element)
			}
		}
	}
}

func writeQueryFilter(builder *XMLBuilder, query CalendarQuery) {
	builder.WriteStartElement("C:filter").
		WriteStartElement("C:comp-filter", "name", "VCALENDAR")

	writeFilterContent(builder, query.Filter, query.TimeRange)

	builder.WriteEndElement("C:comp-filter").
		WriteEndElement("C:filter")
}

func writeFilterContent(builder *XMLBuilder, filter Filter, timeRange *TimeRange) {
	if filter.Component != "" && filter.Component != "VCALENDAR" {
		writeComponentFilter(builder, filter)
	} else if filter.Component == "VCALENDAR" {
		writeVCalendarFilter(builder, filter)
	} else if len(filter.CompFilters) > 0 {
		writeCompFilters(builder, filter.CompFilters)
	} else if timeRange != nil {
		writeEventTimeRange(builder, timeRange)
	}
}

func writeVCalendarFilter(builder *XMLBuilder, filter Filter) {
	writePropFilters(builder, filter.Props)
	if filter.TimeRange != nil {
		writeTimeRange(builder, filter.TimeRange)
	}
	writeCompFilters(builder, filter.CompFilters)
}

func writeCompFilters(builder *XMLBuilder, compFilters []Filter) {
	for _, compFilter := range compFilters {
		writeComponentFilter(builder, compFilter)
	}
}

func writeEventTimeRange(builder *XMLBuilder, timeRange *TimeRange) {
	builder.WriteStartElement("C:comp-filter", "name", "VEVENT")
	writeTimeRange(builder, timeRange)
	builder.WriteEndElement("C:comp-filter")
}

func buildCalendarQueryXML(query CalendarQuery) ([]byte, error) {
	estimatedSize := calculateQueryXMLSize(query)
	builder := NewXMLBuilder(estimatedSize)

	builder.WriteHeader().
		WriteStartElement("C:calendar-query",
			"xmlns:D", "DAV:",
			"xmlns:C", "urn:ietf:params:xml:ns:caldav").
		WriteStartElement("D:prop")

	writeQueryProperties(builder, query.Properties)
	builder.WriteEndElement("D:prop")

	if needsFilter(query) {
		writeQueryFilter(builder, query)
	}

	builder.WriteEndElement("C:calendar-query")

	return builder.Bytes(), nil
}

func writeComponentFilter(builder *XMLBuilder, filter Filter) {
	builder.WriteStartElement("C:comp-filter", "name", filter.Component)

	writePropFilters(builder, filter.Props)

	if filter.TimeRange != nil {
		writeTimeRange(builder, filter.TimeRange)
	}

	for _, compFilter := range filter.CompFilters {
		writeComponentFilter(builder, compFilter)
	}

	builder.WriteEndElement("C:comp-filter")
}

func writePropFilters(builder *XMLBuilder, props []PropFilter) {
	for _, propFilter := range props {
		builder.WriteStartElement("C:prop-filter", "name", propFilter.Name)

		if propFilter.TextMatch != nil {
			attrs := make([]string, 0, 4)

			if propFilter.TextMatch.Collation != "" {
				attrs = append(attrs, "collation", propFilter.TextMatch.Collation)
			}
			if propFilter.TextMatch.NegateCondition {
				attrs = append(attrs, "negate-condition", "yes")
			}

			builder.WriteStartElement("C:text-match", attrs...)
			builder.WriteText(propFilter.TextMatch.Value)
			builder.WriteEndElement("C:text-match")
		}

		if propFilter.TimeRange != nil {
			writeTimeRange(builder, propFilter.TimeRange)
		}

		builder.WriteEndElement("C:prop-filter")
	}
}

func writeTimeRange(builder *XMLBuilder, tr *TimeRange) {
	startStr := formatTimeForCalDAV(tr.Start)
	endStr := formatTimeForCalDAV(tr.End)
	builder.WriteSelfClosingElement("C:time-range", "start", startStr, "end", endStr)
}
