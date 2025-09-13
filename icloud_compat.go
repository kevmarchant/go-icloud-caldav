package caldav

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type ServerCapability string

const (
	CapCalendarAccess           ServerCapability = "calendar-access"
	CapCalendarSchedule         ServerCapability = "calendar-schedule"
	CapCalendarAutoSchedule     ServerCapability = "calendar-auto-schedule"
	CapCalendarAvailability     ServerCapability = "calendar-availability"
	CapCalendarProxy            ServerCapability = "calendar-proxy"
	CapCalendarQueryExtended    ServerCapability = "calendar-query-extended"
	CapCalendarManagedAttach    ServerCapability = "calendar-managed-attachments"
	CapCalendarNoInstance       ServerCapability = "calendar-no-instance"
	CapInboxAvailability        ServerCapability = "inbox-availability"
	CapCalendarServerSharing    ServerCapability = "calendarserver-sharing"
	CapCalendarServerSubscribed ServerCapability = "calendarserver-subscribed"
	CapCalendarServerHomeSync   ServerCapability = "calendarserver-home-sync"
	CapCalendarServerComments   ServerCapability = "calendarserver-private-comments"
	CapVJournal                 ServerCapability = "vjournal"
	CapVResource                ServerCapability = "vresource"
	CapVAvailability            ServerCapability = "vavailability"
)

type ServerType string

const (
	ServerTypeGeneric   ServerType = "generic"
	ServerTypeICloud    ServerType = "icloud"
	ServerTypeGoogle    ServerType = "google"
	ServerTypeNextcloud ServerType = "nextcloud"
	ServerTypeZimbra    ServerType = "zimbra"
)

type ServerCompatibility struct {
	Type         ServerType
	Capabilities map[ServerCapability]bool
	ServerString string
	Version      string
}

func (c *CalDAVClient) DetectServerType(ctx context.Context) (*ServerCompatibility, error) {
	req, err := c.prepareRequest(ctx, "OPTIONS", "/", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	compat := &ServerCompatibility{
		Type:         ServerTypeGeneric,
		Capabilities: make(map[ServerCapability]bool),
	}

	davHeader := resp.Header.Get("DAV")
	serverHeader := resp.Header.Get("Server")
	compat.ServerString = serverHeader

	if strings.Contains(strings.ToLower(serverHeader), "icloud") ||
		strings.Contains(strings.ToLower(serverHeader), "apple") ||
		strings.Contains(strings.ToLower(c.baseURL), "icloud.com") {
		compat.Type = ServerTypeICloud
		c.populateICloudCapabilities(compat, davHeader)
	} else if strings.Contains(strings.ToLower(serverHeader), "google") {
		compat.Type = ServerTypeGoogle
		c.populateGoogleCapabilities(compat, davHeader)
	} else if strings.Contains(strings.ToLower(serverHeader), "nextcloud") {
		compat.Type = ServerTypeNextcloud
		c.populateNextcloudCapabilities(compat, davHeader)
	} else {
		c.populateGenericCapabilities(compat, davHeader)
	}

	return compat, nil
}

func (c *CalDAVClient) populateICloudCapabilities(compat *ServerCompatibility, davHeader string) {
	compat.Capabilities[CapCalendarAccess] = true
	compat.Capabilities[CapCalendarProxy] = true
	compat.Capabilities[CapCalendarQueryExtended] = true
	compat.Capabilities[CapCalendarManagedAttach] = true
	compat.Capabilities[CapCalendarNoInstance] = true
	compat.Capabilities[CapCalendarServerSharing] = true
	compat.Capabilities[CapCalendarServerSubscribed] = true
	compat.Capabilities[CapCalendarServerHomeSync] = true
	compat.Capabilities[CapCalendarServerComments] = true

	compat.Capabilities[CapCalendarSchedule] = false
	compat.Capabilities[CapCalendarAutoSchedule] = false
	compat.Capabilities[CapCalendarAvailability] = false
	compat.Capabilities[CapInboxAvailability] = false
	compat.Capabilities[CapVJournal] = false
	compat.Capabilities[CapVResource] = false
	compat.Capabilities[CapVAvailability] = false
}

func (c *CalDAVClient) populateGoogleCapabilities(compat *ServerCompatibility, davHeader string) {
	compat.Capabilities[CapCalendarAccess] = true
	compat.Capabilities[CapCalendarProxy] = true
	compat.Capabilities[CapCalendarQueryExtended] = true

	compat.Capabilities[CapCalendarSchedule] = false
	compat.Capabilities[CapCalendarManagedAttach] = false
	compat.Capabilities[CapVJournal] = false
	compat.Capabilities[CapVResource] = false
}

func (c *CalDAVClient) populateNextcloudCapabilities(compat *ServerCompatibility, davHeader string) {
	compat.Capabilities[CapCalendarAccess] = true
	compat.Capabilities[CapCalendarProxy] = true
	compat.Capabilities[CapCalendarQueryExtended] = true
	compat.Capabilities[CapCalendarSchedule] = true
	compat.Capabilities[CapCalendarAutoSchedule] = true

	compat.Capabilities[CapCalendarManagedAttach] = false
	compat.Capabilities[CapVResource] = false
}

func (c *CalDAVClient) populateGenericCapabilities(compat *ServerCompatibility, davHeader string) {
	compat.Capabilities[CapCalendarAccess] = strings.Contains(davHeader, "calendar-access")
	compat.Capabilities[CapCalendarSchedule] = strings.Contains(davHeader, "calendar-schedule")
	compat.Capabilities[CapCalendarAutoSchedule] = strings.Contains(davHeader, "calendar-auto-schedule")
	compat.Capabilities[CapCalendarAvailability] = strings.Contains(davHeader, "calendar-availability")
	compat.Capabilities[CapCalendarProxy] = strings.Contains(davHeader, "calendar-proxy")
	compat.Capabilities[CapCalendarQueryExtended] = strings.Contains(davHeader, "calendar-query-extended")
	compat.Capabilities[CapCalendarManagedAttach] = strings.Contains(davHeader, "calendar-managed-attachments")
	compat.Capabilities[CapCalendarNoInstance] = strings.Contains(davHeader, "calendar-no-instance")
	compat.Capabilities[CapInboxAvailability] = strings.Contains(davHeader, "inbox-availability")
}

func (c *CalDAVClient) IsICloudServer(ctx context.Context) (bool, error) {
	compat, err := c.DetectServerType(ctx)
	if err != nil {
		return false, err
	}
	return compat.Type == ServerTypeICloud, nil
}

func (c *CalDAVClient) GetSupportedFeatures(ctx context.Context) (map[ServerCapability]bool, error) {
	compat, err := c.DetectServerType(ctx)
	if err != nil {
		return nil, err
	}
	return compat.Capabilities, nil
}

func (c *CalDAVClient) SupportsFeature(ctx context.Context, capability ServerCapability) (bool, error) {
	features, err := c.GetSupportedFeatures(ctx)
	if err != nil {
		return false, err
	}
	return features[capability], nil
}

func (c *CalDAVClient) GetServerCompatibility(ctx context.Context) (*ServerCompatibility, error) {
	return c.DetectServerType(ctx)
}

func (c *CalDAVClient) ConfigureForICloud() {
	c.SetTimeout(30)

	if !strings.HasSuffix(c.baseURL, "/") {
		c.baseURL = c.baseURL + "/"
	}

	c.httpClient.Timeout = 30 * time.Second

	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 10
		transport.IdleConnTimeout = 90 * time.Second
	}
}

func IsFeatureSupportedByICloud(capability ServerCapability) bool {
	switch capability {
	case CapCalendarAccess,
		CapCalendarProxy,
		CapCalendarQueryExtended,
		CapCalendarManagedAttach,
		CapCalendarNoInstance,
		CapCalendarServerSharing,
		CapCalendarServerSubscribed,
		CapCalendarServerHomeSync,
		CapCalendarServerComments:
		return true
	default:
		return false
	}
}

func GetICloudSpecificHeaders() map[string]string {
	return map[string]string{
		"X-Apple-Calendar-User-Agent": "go-icloud-caldav/1.0",
		"Accept":                      "text/calendar, text/xml, application/xml",
		"Accept-Language":             "en-US,en;q=0.9",
	}
}

func (c *CalDAVClient) AddICloudHeaders(req *http.Request) {
	headers := GetICloudSpecificHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}
