package caldav

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// ParseICalendar parses iCalendar data and returns structured ParsedCalendarData.
func ParseICalendar(data string) (*ParsedCalendarData, error) {
	parser := &icalParser{
		scanner: bufio.NewScanner(strings.NewReader(data)),
		result: &ParsedCalendarData{
			Events:           make([]ParsedEvent, 0),
			Todos:            make([]ParsedTodo, 0),
			Journals:         make([]ParsedJournal, 0),
			FreeBusy:         make([]ParsedFreeBusy, 0),
			TimeZones:        make([]ParsedTimeZone, 0),
			Alarms:           make([]ParsedAlarm, 0),
			CustomProperties: make(map[string]string),
		},
	}

	return parser.parse()
}

type icalParser struct {
	scanner         *bufio.Scanner
	result          *ParsedCalendarData
	currentEvent    *ParsedEvent
	currentTodo     *ParsedTodo
	currentJournal  *ParsedJournal
	currentFreeBusy *ParsedFreeBusy
	currentTimeZone *ParsedTimeZone
	currentAlarm    *ParsedAlarm
	inEvent         bool
	inTodo          bool
	inJournal       bool
	inFreeBusy      bool
	inTimeZone      bool
	inAlarm         bool
}

func (p *icalParser) parse() (*ParsedCalendarData, error) {
	var currentLine string

	for {
		var line string

		// Use buffered line if available
		if currentLine != "" {
			line = currentLine
			currentLine = ""
		} else if p.scanner.Scan() {
			line = p.scanner.Text()
		} else {
			break
		}

		// Handle line folding (RFC 5545)
		for p.scanner.Scan() {
			nextLine := p.scanner.Text()
			if len(nextLine) > 0 && (nextLine[0] == ' ' || nextLine[0] == '\t') {
				line += strings.TrimLeft(nextLine, " \t")
			} else {
				currentLine = nextLine
				break
			}
		}

		if line == "" {
			continue
		}

		if err := p.parseLine(line); err != nil {
			return nil, err
		}
	}

	if err := p.scanner.Err(); err != nil {
		return nil, wrapErrorWithType("ical.parse", ErrorTypeInvalidResponse, err)
	}

	return p.result, nil
}

func (p *icalParser) parseLine(line string) error {
	colonIndex := strings.Index(line, ":")
	if colonIndex == -1 {
		return nil // Skip malformed lines
	}

	propertyPart := line[:colonIndex]
	value := line[colonIndex+1:]

	// Parse property name and parameters
	property, params := p.parseProperty(propertyPart)

	switch property {
	case "BEGIN":
		p.handleBegin(value)
	case "END":
		p.handleEnd(value)
	case "VERSION":
		p.result.Version = value
	case "PRODID":
		p.result.ProdID = value
	case "CALSCALE":
		p.result.CalScale = value
	case "METHOD":
		p.result.Method = value
	default:
		p.handleProperty(property, value, params)
	}

	return nil
}

func (p *icalParser) parseProperty(propertyPart string) (string, map[string]string) {
	parts := strings.Split(propertyPart, ";")
	property := parts[0]
	params := make(map[string]string)

	for i := 1; i < len(parts); i++ {
		paramParts := strings.SplitN(parts[i], "=", 2)
		if len(paramParts) == 2 {
			params[paramParts[0]] = strings.Trim(paramParts[1], "\"")
		}
	}

	return property, params
}

func (p *icalParser) handleBegin(component string) {
	switch component {
	case "VEVENT":
		p.inEvent = true
		p.currentEvent = &ParsedEvent{
			Attendees:        make([]ParsedAttendee, 0),
			Categories:       make([]string, 0),
			ExceptionDates:   make([]time.Time, 0),
			Alarms:           make([]ParsedAlarm, 0),
			CustomProperties: make(map[string]string),
		}
	case "VTODO":
		p.inTodo = true
		p.currentTodo = &ParsedTodo{
			Categories:       make([]string, 0),
			CustomProperties: make(map[string]string),
		}
	case "VJOURNAL":
		p.inJournal = true
		p.currentJournal = &ParsedJournal{
			Categories:       make([]string, 0),
			CustomProperties: make(map[string]string),
		}
	case "VFREEBUSY":
		p.inFreeBusy = true
		p.currentFreeBusy = &ParsedFreeBusy{
			Attendees:        make([]ParsedAttendee, 0),
			FreeBusy:         make([]FreeBusyPeriod, 0),
			CustomProperties: make(map[string]string),
		}
	case "VTIMEZONE":
		p.inTimeZone = true
		p.currentTimeZone = &ParsedTimeZone{
			CustomProperties: make(map[string]string),
		}
	case "VALARM":
		p.inAlarm = true
		p.currentAlarm = &ParsedAlarm{
			Attendees:        make([]ParsedAttendee, 0),
			CustomProperties: make(map[string]string),
		}
	}
}

func (p *icalParser) handleEnd(component string) {
	switch component {
	case "VEVENT":
		if p.inEvent && p.currentEvent != nil {
			p.result.Events = append(p.result.Events, *p.currentEvent)
			p.currentEvent = nil
			p.inEvent = false
		}
	case "VTODO":
		if p.inTodo && p.currentTodo != nil {
			p.result.Todos = append(p.result.Todos, *p.currentTodo)
			p.currentTodo = nil
			p.inTodo = false
		}
	case "VJOURNAL":
		if p.inJournal && p.currentJournal != nil {
			p.result.Journals = append(p.result.Journals, *p.currentJournal)
			p.currentJournal = nil
			p.inJournal = false
		}
	case "VFREEBUSY":
		if p.inFreeBusy && p.currentFreeBusy != nil {
			p.result.FreeBusy = append(p.result.FreeBusy, *p.currentFreeBusy)
			p.currentFreeBusy = nil
			p.inFreeBusy = false
		}
	case "VTIMEZONE":
		if p.inTimeZone && p.currentTimeZone != nil {
			p.result.TimeZones = append(p.result.TimeZones, *p.currentTimeZone)
			p.currentTimeZone = nil
			p.inTimeZone = false
		}
	case "VALARM":
		if p.inAlarm && p.currentAlarm != nil {
			if p.inEvent && p.currentEvent != nil {
				p.currentEvent.Alarms = append(p.currentEvent.Alarms, *p.currentAlarm)
			} else {
				p.result.Alarms = append(p.result.Alarms, *p.currentAlarm)
			}
			p.currentAlarm = nil
			p.inAlarm = false
		}
	}
}

func (p *icalParser) handleProperty(property, value string, params map[string]string) {
	// Handle alarms first since they can be nested in events
	if p.inAlarm && p.currentAlarm != nil {
		p.handleAlarmProperty(property, value, params)
	} else if p.inEvent && p.currentEvent != nil {
		p.handleEventProperty(property, value, params)
	} else if p.inTodo && p.currentTodo != nil {
		p.handleTodoProperty(property, value, params)
	} else if p.inJournal && p.currentJournal != nil {
		p.handleJournalProperty(property, value, params)
	} else if p.inFreeBusy && p.currentFreeBusy != nil {
		p.handleFreeBusyProperty(property, value, params)
	} else if p.inTimeZone && p.currentTimeZone != nil {
		p.handleTimeZoneProperty(property, value, params)
	} else {
		// Global custom properties
		p.result.CustomProperties[property] = value
	}
}

func (p *icalParser) handleEventProperty(property, value string, params map[string]string) {
	switch property {
	case "UID", "DURATION", "SUMMARY", "LOCATION", "STATUS", "TRANSP", "CLASS", "URL", "RRULE":
		p.setEventStringProperty(property, value)
	case "DTSTAMP", "DTSTART", "DTEND", "RECURRENCE-ID", "CREATED", "LAST-MODIFIED":
		p.setEventTimeProperty(property, value, params)
	case "DESCRIPTION":
		p.appendEventDescription(value)
	case "CATEGORIES":
		p.currentEvent.Categories = append(p.currentEvent.Categories, strings.Split(value, ",")...)
	case "ORGANIZER":
		p.currentEvent.Organizer = p.parseOrganizer(value, params)
	case "ATTENDEE":
		p.currentEvent.Attendees = append(p.currentEvent.Attendees, p.parseAttendee(value, params))
	case "EXDATE":
		if t := p.parseTime(value, params); t != nil {
			p.currentEvent.ExceptionDates = append(p.currentEvent.ExceptionDates, *t)
		}
	case "SEQUENCE", "PRIORITY":
		p.setEventIntProperty(property, value)
	case "GEO":
		p.currentEvent.GeoLocation = p.parseGeo(value)
	default:
		p.currentEvent.CustomProperties[property] = value
	}
}

func (p *icalParser) setEventStringProperty(property, value string) {
	switch property {
	case "UID":
		p.currentEvent.UID = value
	case "DURATION":
		p.currentEvent.Duration = value
	case "SUMMARY":
		p.currentEvent.Summary = value
	case "LOCATION":
		p.currentEvent.Location = value
	case "STATUS":
		p.currentEvent.Status = value
	case "TRANSP":
		p.currentEvent.Transparency = value
	case "CLASS":
		p.currentEvent.Class = value
	case "URL":
		p.currentEvent.URL = value
	case "RRULE":
		p.currentEvent.RecurrenceRule = value
	}
}

func (p *icalParser) setEventTimeProperty(property, value string, params map[string]string) {
	t := p.parseTime(value, params)
	if t == nil {
		return
	}

	switch property {
	case "DTSTAMP":
		p.currentEvent.DTStamp = t
	case "DTSTART":
		p.currentEvent.DTStart = t
	case "DTEND":
		p.currentEvent.DTEnd = t
	case "RECURRENCE-ID":
		p.currentEvent.RecurrenceID = t
	case "CREATED":
		p.currentEvent.Created = t
	case "LAST-MODIFIED":
		p.currentEvent.LastModified = t
	}
}

func (p *icalParser) setEventIntProperty(property, value string) {
	switch property {
	case "SEQUENCE":
		p.currentEvent.Sequence = p.parseInt(value)
	case "PRIORITY":
		p.currentEvent.Priority = p.parseInt(value)
	}
}

func (p *icalParser) appendEventDescription(value string) {
	if p.currentEvent.Description == "" {
		p.currentEvent.Description = value
	} else {
		p.currentEvent.Description += value
	}
}

func (p *icalParser) handleTodoProperty(property, value string, params map[string]string) {
	switch property {
	case "UID":
		p.currentTodo.UID = value
	case "DTSTAMP":
		if t := p.parseTime(value, params); t != nil {
			p.currentTodo.DTStamp = t
		}
	case "DTSTART":
		if t := p.parseTime(value, params); t != nil {
			p.currentTodo.DTStart = t
		}
	case "DUE":
		if t := p.parseTime(value, params); t != nil {
			p.currentTodo.Due = t
		}
	case "COMPLETED":
		if t := p.parseTime(value, params); t != nil {
			p.currentTodo.Completed = t
		}
	case "SUMMARY":
		p.currentTodo.Summary = value
	case "DESCRIPTION":
		p.currentTodo.Description = value
	case "STATUS":
		p.currentTodo.Status = value
	case "PERCENT-COMPLETE":
		p.currentTodo.PercentComplete = p.parseInt(value)
	case "PRIORITY":
		p.currentTodo.Priority = p.parseInt(value)
	case "CATEGORIES":
		p.currentTodo.Categories = append(p.currentTodo.Categories, strings.Split(value, ",")...)
	case "CREATED":
		if t := p.parseTime(value, params); t != nil {
			p.currentTodo.Created = t
		}
	case "LAST-MODIFIED":
		if t := p.parseTime(value, params); t != nil {
			p.currentTodo.LastModified = t
		}
	case "SEQUENCE":
		p.currentTodo.Sequence = p.parseInt(value)
	case "CLASS":
		p.currentTodo.Class = value
	case "URL":
		p.currentTodo.URL = value
	default:
		p.currentTodo.CustomProperties[property] = value
	}
}

func (p *icalParser) handleJournalProperty(property, value string, params map[string]string) {
	switch property {
	case "UID":
		p.currentJournal.UID = value
	case "DTSTAMP":
		if t := p.parseTime(value, params); t != nil {
			p.currentJournal.DTStamp = t
		}
	case "DTSTART":
		if t := p.parseTime(value, params); t != nil {
			p.currentJournal.DTStart = t
		}
	case "SUMMARY":
		p.currentJournal.Summary = value
	case "DESCRIPTION":
		p.currentJournal.Description = value
	case "STATUS":
		p.currentJournal.Status = value
	case "CATEGORIES":
		p.currentJournal.Categories = append(p.currentJournal.Categories, strings.Split(value, ",")...)
	case "CREATED":
		if t := p.parseTime(value, params); t != nil {
			p.currentJournal.Created = t
		}
	case "LAST-MODIFIED":
		if t := p.parseTime(value, params); t != nil {
			p.currentJournal.LastModified = t
		}
	case "SEQUENCE":
		p.currentJournal.Sequence = p.parseInt(value)
	case "CLASS":
		p.currentJournal.Class = value
	default:
		p.currentJournal.CustomProperties[property] = value
	}
}

func (p *icalParser) handleFreeBusyProperty(property, value string, params map[string]string) {
	switch property {
	case "UID":
		p.currentFreeBusy.UID = value
	case "DTSTAMP":
		if t := p.parseTime(value, params); t != nil {
			p.currentFreeBusy.DTStamp = t
		}
	case "DTSTART":
		if t := p.parseTime(value, params); t != nil {
			p.currentFreeBusy.DTStart = t
		}
	case "DTEND":
		if t := p.parseTime(value, params); t != nil {
			p.currentFreeBusy.DTEnd = t
		}
	case "ORGANIZER":
		p.currentFreeBusy.Organizer = p.parseOrganizer(value, params)
	case "ATTENDEE":
		p.currentFreeBusy.Attendees = append(p.currentFreeBusy.Attendees, p.parseAttendee(value, params))
	case "FREEBUSY":
		if fb := p.parseFreeBusyPeriod(value, params); fb != nil {
			p.currentFreeBusy.FreeBusy = append(p.currentFreeBusy.FreeBusy, *fb)
		}
	default:
		p.currentFreeBusy.CustomProperties[property] = value
	}
}

func (p *icalParser) handleTimeZoneProperty(property, value string, params map[string]string) {
	switch property {
	case "TZID":
		p.currentTimeZone.TZID = value
	default:
		p.currentTimeZone.CustomProperties[property] = value
	}
}

func (p *icalParser) handleAlarmProperty(property, value string, params map[string]string) {
	switch property {
	case "ACTION":
		p.currentAlarm.Action = value
	case "TRIGGER":
		p.currentAlarm.Trigger = value
	case "DURATION":
		p.currentAlarm.Duration = value
	case "REPEAT":
		p.currentAlarm.Repeat = p.parseInt(value)
	case "DESCRIPTION":
		p.currentAlarm.Description = value
	case "SUMMARY":
		p.currentAlarm.Summary = value
	case "ATTENDEE":
		p.currentAlarm.Attendees = append(p.currentAlarm.Attendees, p.parseAttendee(value, params))
	default:
		p.currentAlarm.CustomProperties[property] = value
	}
}

func (p *icalParser) parseTime(value string, params map[string]string) *time.Time {
	// Basic parsing of common iCalendar time formats
	formats := []string{
		"20060102T150405Z", // UTC time
		"20060102T150405",  // Local time
		"20060102",         // Date only
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return &t
		}
	}

	return nil
}

func (p *icalParser) parseInt(value string) int {
	var result int
	_, _ = fmt.Sscanf(value, "%d", &result)
	return result
}

func (p *icalParser) parseOrganizer(value string, params map[string]string) ParsedOrganizer {
	org := ParsedOrganizer{
		Value:        value,
		CustomParams: make(map[string]string),
	}

	for key, val := range params {
		switch key {
		case "CN":
			org.CN = val
		case "EMAIL":
			org.Email = val
		case "DIR":
			org.Dir = val
		case "SENT-BY":
			org.SentBy = val
		default:
			org.CustomParams[key] = val
		}
	}

	// Extract email from mailto: URI if present
	if strings.HasPrefix(strings.ToLower(value), "mailto:") {
		org.Email = value[7:]
	}

	return org
}

func (p *icalParser) parseAttendee(value string, params map[string]string) ParsedAttendee {
	att := ParsedAttendee{
		Value:        value,
		CustomParams: make(map[string]string),
	}

	for key, val := range params {
		switch key {
		case "CN":
			att.CN = val
		case "EMAIL":
			att.Email = val
		case "ROLE":
			att.Role = val
		case "PARTSTAT":
			att.PartStat = val
		case "RSVP":
			att.RSVP = strings.ToUpper(val) == "TRUE"
		case "CUTYPE":
			att.CUType = val
		case "MEMBER":
			att.Member = val
		case "DELEGATED-TO":
			att.DelegatedTo = val
		case "DELEGATED-FROM":
			att.DelegatedFrom = val
		case "DIR":
			att.Dir = val
		case "SENT-BY":
			att.SentBy = val
		default:
			att.CustomParams[key] = val
		}
	}

	// Extract email from mailto: URI if present
	if strings.HasPrefix(strings.ToLower(value), "mailto:") {
		att.Email = value[7:]
	}

	return att
}

func (p *icalParser) parseGeo(value string) *GeoLocation {
	parts := strings.Split(value, ";")
	if len(parts) != 2 {
		return nil
	}

	var lat, lon float64
	if _, err := fmt.Sscanf(parts[0], "%f", &lat); err != nil {
		return nil
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &lon); err != nil {
		return nil
	}

	return &GeoLocation{
		Latitude:  lat,
		Longitude: lon,
	}
}

func (p *icalParser) parseFreeBusyPeriod(value string, params map[string]string) *FreeBusyPeriod {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return nil
	}

	start := p.parseTime(parts[0], nil)
	end := p.parseTime(parts[1], nil)

	if start == nil || end == nil {
		return nil
	}

	fb := &FreeBusyPeriod{
		Start:  *start,
		End:    *end,
		FBType: params["FBTYPE"],
	}

	if fb.FBType == "" {
		fb.FBType = "BUSY"
	}

	return fb
}
