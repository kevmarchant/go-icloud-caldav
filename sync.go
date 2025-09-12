package caldav

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
	"time"
)

type syncMultiStatus struct {
	XMLName   xml.Name      `xml:"multistatus"`
	SyncToken string        `xml:"sync-token"`
	Responses []xmlResponse `xml:"response"`
}

type SyncToken struct {
	Token     string
	Timestamp time.Time
	Valid     bool
}

type SyncRequest struct {
	CalendarURL string
	SyncToken   string
	SyncLevel   int
	Properties  []string
	Limit       int
}

type SyncResponse struct {
	SyncToken    string
	Changes      []SyncChange
	MoreToSync   bool
	TotalChanges int
}

type SyncChange struct {
	Href         string
	Status       int
	ETag         string
	CalendarData string
	Deleted      bool
	Properties   map[string]string
}

type SyncChangeType int

const (
	SyncChangeTypeNew SyncChangeType = iota
	SyncChangeTypeModified
	SyncChangeTypeDeleted
)

func (c *SyncChange) ChangeType() SyncChangeType {
	if c.Deleted {
		return SyncChangeTypeDeleted
	}
	if c.CalendarData != "" && c.ETag != "" {
		return SyncChangeTypeModified
	}
	return SyncChangeTypeNew
}

func (c *CalDAVClient) SyncCalendar(ctx context.Context, req *SyncRequest) (*SyncResponse, error) {
	if req.CalendarURL == "" {
		return nil, newTypedError("SyncCalendar", ErrorTypeValidation, "calendar URL is required for sync", nil)
	}

	xmlBody := buildSyncCollectionXML(req)

	resp, err := c.report(ctx, req.CalendarURL, []byte(xmlBody))
	if err != nil {
		return nil, wrapErrorWithType("SyncCalendar", ErrorTypeNetwork, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		return nil, newTypedError("SyncCalendar", ErrorTypeServer, "unexpected status code", nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("SyncCalendar", ErrorTypeNetwork, err)
	}

	return parseSyncResponse(body)
}

func (c *CalDAVClient) InitialSync(ctx context.Context, calendarURL string) (*SyncResponse, error) {
	return c.SyncCalendar(ctx, &SyncRequest{
		CalendarURL: calendarURL,
		SyncToken:   "",
		SyncLevel:   1,
		Properties: []string{
			"getetag",
			"calendar-data",
		},
	})
}

func (c *CalDAVClient) IncrementalSync(ctx context.Context, calendarURL string, syncToken string) (*SyncResponse, error) {
	if syncToken == "" {
		return nil, newTypedError("IncrementalSync", ErrorTypeValidation, "sync token is required for incremental sync", nil)
	}

	return c.SyncCalendar(ctx, &SyncRequest{
		CalendarURL: calendarURL,
		SyncToken:   syncToken,
		SyncLevel:   1,
		Properties: []string{
			"getetag",
			"calendar-data",
		},
	})
}

func (c *CalDAVClient) SyncAllCalendars(ctx context.Context, syncTokens map[string]string) (map[string]*SyncResponse, error) {
	calendars, err := c.DiscoverCalendars(ctx)
	if err != nil {
		return nil, wrapErrorWithType("SyncAllCalendars", ErrorTypeInvalidRequest, err)
	}

	results := make(map[string]*SyncResponse)

	for _, cal := range calendars {
		var syncResp *SyncResponse
		token, hasToken := syncTokens[cal.Href]

		if hasToken && token != "" {
			syncResp, err = c.IncrementalSync(ctx, cal.Href, token)
			if err != nil {
				syncResp, err = c.InitialSync(ctx, cal.Href)
			}
		} else {
			syncResp, err = c.InitialSync(ctx, cal.Href)
		}

		if err != nil {
			if c.logger != nil {
				c.logger.Debug("Error syncing calendar", "name", cal.DisplayName, "error", err)
			}
			continue
		}

		results[cal.Href] = syncResp
	}

	return results, nil
}

func buildSyncCollectionXML(req *SyncRequest) string {
	var buf strings.Builder

	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	if req.SyncToken != "" {
		buf.WriteString(`<D:sync-token>`)
		buf.WriteString(xmlEscape(req.SyncToken))
		buf.WriteString(`</D:sync-token>`)
	} else {
		buf.WriteString(`<D:sync-token/>`)
	}

	buf.WriteString(`<D:sync-level>`)
	if req.SyncLevel > 0 {
		buf.WriteString("1")
	} else {
		buf.WriteString("infinite")
	}
	buf.WriteString(`</D:sync-level>`)

	if req.Limit > 0 {
		buf.WriteString(`<D:limit><D:nresults>`)
		buf.WriteString(intToString(req.Limit))
		buf.WriteString(`</D:nresults></D:limit>`)
	}

	buf.WriteString(`<D:prop>`)
	for _, prop := range req.Properties {
		switch prop {
		case "getetag":
			buf.WriteString(`<D:getetag/>`)
		case "calendar-data":
			buf.WriteString(`<C:calendar-data/>`)
		case "getcontenttype":
			buf.WriteString(`<D:getcontenttype/>`)
		case "displayname":
			buf.WriteString(`<D:displayname/>`)
		default:
			buf.WriteString(`<D:`)
			buf.WriteString(prop)
			buf.WriteString(`/>`)
		}
	}
	buf.WriteString(`</D:prop>`)

	buf.WriteString(`</D:sync-collection>`)

	return buf.String()
}

func parseSyncResponse(body []byte) (*SyncResponse, error) {
	var multistatus syncMultiStatus
	if err := xml.Unmarshal(body, &multistatus); err != nil {
		return nil, wrapErrorWithType("parseSyncResponse", ErrorTypeInvalidXML, err)
	}

	resp := &SyncResponse{
		SyncToken: multistatus.SyncToken,
		Changes:   make([]SyncChange, 0),
	}

	for _, response := range multistatus.Responses {
		change := SyncChange{
			Href:       response.Href,
			Properties: make(map[string]string),
		}

		for _, propstat := range response.Propstats {
			status := parseStatusCode(propstat.Status)

			if status == 404 {
				change.Deleted = true
				change.Status = status
				continue
			}

			change.Status = status

			if propstat.Prop.GetETag != "" {
				change.ETag = propstat.Prop.GetETag
				change.Properties["getetag"] = propstat.Prop.GetETag
			}

			if propstat.Prop.CalendarData != "" {
				change.CalendarData = propstat.Prop.CalendarData
				change.Properties["calendar-data"] = propstat.Prop.CalendarData
			}

			if propstat.Prop.DisplayName != "" {
				change.Properties["displayname"] = propstat.Prop.DisplayName
			}

			if propstat.Prop.GetContentType != "" {
				change.Properties["getcontenttype"] = propstat.Prop.GetContentType
			}
		}

		resp.Changes = append(resp.Changes, change)
	}

	resp.TotalChanges = len(resp.Changes)

	return resp, nil
}

func (resp *SyncResponse) GetNewItems() []SyncChange {
	var items []SyncChange
	for _, change := range resp.Changes {
		if change.ChangeType() == SyncChangeTypeNew {
			items = append(items, change)
		}
	}
	return items
}

func (resp *SyncResponse) GetModifiedItems() []SyncChange {
	var items []SyncChange
	for _, change := range resp.Changes {
		if change.ChangeType() == SyncChangeTypeModified {
			items = append(items, change)
		}
	}
	return items
}

func (resp *SyncResponse) GetDeletedItems() []SyncChange {
	var items []SyncChange
	for _, change := range resp.Changes {
		if change.ChangeType() == SyncChangeTypeDeleted {
			items = append(items, change)
		}
	}
	return items
}

func (resp *SyncResponse) HasChanges() bool {
	return len(resp.Changes) > 0
}

func intToString(n int) string {
	if n <= 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{'0' + byte(n%10)}, result...)
		n /= 10
	}

	return string(result)
}
