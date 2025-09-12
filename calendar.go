package caldav

import (
	"context"
	"io"
)

// FindCurrentUserPrincipal discovers the current user's principal path.
// This is typically the first step in calendar discovery.
// Returns the principal path (e.g., "/123456789/principal/").
func (c *CalDAVClient) FindCurrentUserPrincipal(ctx context.Context) (string, error) {
	props := []string{"current-user-principal"}
	xmlBody, err := buildPropfindXML(props)
	if err != nil {
		return "", wrapErrorWithType("principal.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.propfind(ctx, "/", "0", xmlBody)
	if err != nil {
		return "", wrapError("principal.execute", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		body, _ := io.ReadAll(resp.Body)
		return "", newCalDAVError("principal", resp.StatusCode, string(body))
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return "", wrapErrorWithType("principal.parse", ErrorTypeInvalidResponse, err)
	}

	principal := extractPrincipalFromResponse(msResp)
	if principal == "" {
		return "", newTypedError("principal", ErrorTypeNotFound, "no principal found in response", ErrNotFound)
	}

	return principal, nil
}

// FindCalendarHomeSet discovers the calendar home collection for a principal.
// The principalPath is typically obtained from FindCurrentUserPrincipal.
// Returns the calendar home URL where calendars are stored.
func (c *CalDAVClient) FindCalendarHomeSet(ctx context.Context, principalPath string) (string, error) {
	props := []string{"calendar-home-set"}
	xmlBody, err := buildPropfindXML(props)
	if err != nil {
		return "", wrapErrorWithType("principal.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.propfind(ctx, principalPath, "0", xmlBody)
	if err != nil {
		return "", wrapError("principal.execute", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		body, _ := io.ReadAll(resp.Body)
		return "", newCalDAVError("principal", resp.StatusCode, string(body))
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return "", wrapErrorWithType("principal.parse", ErrorTypeInvalidResponse, err)
	}

	homeSet := extractCalendarHomeSetFromResponse(msResp)
	if homeSet == "" {
		return "", newTypedError("calendar-home", ErrorTypeNotFound, "no calendar home set found in response", ErrNotFound)
	}

	return homeSet, nil
}

// FindCalendars lists all calendars in a calendar home collection.
// The calendarHomePath is typically obtained from FindCalendarHomeSet.
// Returns a slice of Calendar objects with their properties.
func (c *CalDAVClient) FindCalendars(ctx context.Context, calendarHomePath string) ([]Calendar, error) {
	props := []string{
		"displayname",
		"resourcetype",
		"calendar-description",
		"calendar-color",
		"supported-calendar-component-set",
		"getctag",
		"getetag",
	}

	xmlBody, err := buildPropfindXML(props)
	if err != nil {
		return nil, wrapErrorWithType("calendars.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.propfind(ctx, calendarHomePath, "1", xmlBody)
	if err != nil {
		return nil, wrapError("calendars.execute", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		body, _ := io.ReadAll(resp.Body)
		return nil, newCalDAVError("calendars", resp.StatusCode, string(body))
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("calendars.parse", ErrorTypeInvalidResponse, err)
	}

	calendars := extractCalendarsFromResponse(msResp)

	return calendars, nil
}

// DiscoverCalendars performs complete calendar discovery for the authenticated user.
// This is a convenience method that calls FindCurrentUserPrincipal,
// FindCalendarHomeSet, and FindCalendars in sequence.
// Returns all calendars accessible to the user.
func (c *CalDAVClient) DiscoverCalendars(ctx context.Context) ([]Calendar, error) {
	principal, err := c.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return nil, wrapError("discover.principal", err)
	}

	homeSet, err := c.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, wrapError("discover.calendar-home", err)
	}

	calendars, err := c.FindCalendars(ctx, homeSet)
	if err != nil {
		return nil, wrapError("discover.calendars", err)
	}

	return calendars, nil
}
