package caldav

import (
	"context"
	"fmt"
	"strings"
)

// FindPrincipal discovers a principal by href.
// This is typically used to resolve user principals.
func (c *CalDAVClient) FindPrincipal(ctx context.Context, principalHref string) (*Principal, error) {
	props := []string{
		"displayname",
		"resourcetype",
		"principal-URL",
		"alternate-URI-set",
		"group-member-set",
		"group-membership",
		"calendar-home-set",
		"calendar-user-address-set",
		"schedule-inbox-URL",
		"schedule-outbox-URL",
	}

	xmlBody, err := buildPropfindXML(props)
	if err != nil {
		return nil, wrapErrorWithType("principal.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.propfind(ctx, principalHref, "0", xmlBody)
	if err != nil {
		return nil, wrapErrorWithType("principal.request", ErrorTypeNetwork, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		return nil, newTypedError("principal.status", ErrorTypeInvalidResponse,
			fmt.Sprintf("expected status 207, got %d", resp.StatusCode), nil)
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("principal.parse", ErrorTypeInvalidResponse, err)
	}

	for _, r := range msResp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 {
				principal := Principal{
					Href:        r.Href,
					DisplayName: ps.Prop.DisplayName,
					Type:        "user",
				}
				return &principal, nil
			}
		}
	}

	return nil, newTypedError("principal.notfound", ErrorTypeNotFound, "principal not found", ErrNotFound)
}

// GetACL retrieves the Access Control List for a resource.
func (c *CalDAVClient) GetACL(ctx context.Context, resourceHref string) (*ACL, error) {
	props := []string{
		"acl",
		"supported-privilege-set",
		"current-user-privilege-set",
		"acl-restrictions",
		"inherited-acl-set",
	}

	xmlBody, err := buildPropfindXML(props)
	if err != nil {
		return nil, wrapErrorWithType("acl.build", ErrorTypeInvalidRequest, err)
	}

	resp, err := c.propfind(ctx, resourceHref, "0", xmlBody)
	if err != nil {
		return nil, wrapErrorWithType("acl.request", ErrorTypeNetwork, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 207 {
		return nil, newTypedError("acl.status", ErrorTypeInvalidResponse,
			fmt.Sprintf("expected status 207, got %d", resp.StatusCode), nil)
	}

	msResp, err := parseMultiStatusResponse(resp.Body)
	if err != nil {
		return nil, wrapErrorWithType("acl.parse", ErrorTypeInvalidResponse, err)
	}

	// For now, return a basic ACL based on current user privileges
	acl := &ACL{
		ACEs: []ACE{},
	}

	for _, r := range msResp.Responses {
		for _, ps := range r.Propstat {
			if ps.Status == 200 {
				// Create ACE from current user privilege set
				if len(ps.Prop.CurrentUserPrivilegeSet) > 0 {
					ace := ACE{
						Principal: Principal{
							Href: c.baseURL + "/principals/" + c.username + "/",
							Type: "user",
						},
						Grant: ps.Prop.CurrentUserPrivilegeSet,
					}
					acl.ACEs = append(acl.ACEs, ace)
				}
			}
		}
	}

	return acl, nil
}

// CheckPermission checks if the current user has a specific privilege on a resource.
func (c *CalDAVClient) CheckPermission(ctx context.Context, resourceHref string, privilege string) (bool, error) {
	acl, err := c.GetACL(ctx, resourceHref)
	if err != nil {
		return false, err
	}

	currentUserHref := c.baseURL + "/principals/" + c.username + "/"

	for _, ace := range acl.ACEs {
		if ace.Principal.Href == currentUserHref {
			for _, grantedPriv := range ace.Grant {
				if grantedPriv == privilege || grantedPriv == "all" {
					return true, nil
				}
			}
			// Check for denied privileges
			for _, deniedPriv := range ace.Deny {
				if deniedPriv == privilege {
					return false, nil
				}
			}
		}
	}

	return false, nil
}

// GetCurrentUserPrivileges returns the privileges of the current user for a resource.
func (c *CalDAVClient) GetCurrentUserPrivileges(ctx context.Context, resourceHref string) ([]string, error) {
	acl, err := c.GetACL(ctx, resourceHref)
	if err != nil {
		return nil, err
	}

	currentUserHref := c.baseURL + "/principals/" + c.username + "/"

	for _, ace := range acl.ACEs {
		if ace.Principal.Href == currentUserHref {
			return ace.Grant, nil
		}
	}

	return []string{}, nil
}

// HasReadAccess checks if the current user has read access to a resource.
func (c *CalDAVClient) HasReadAccess(ctx context.Context, resourceHref string) (bool, error) {
	return c.CheckPermission(ctx, resourceHref, "read")
}

// HasWriteAccess checks if the current user has write access to a resource.
func (c *CalDAVClient) HasWriteAccess(ctx context.Context, resourceHref string) (bool, error) {
	return c.CheckPermission(ctx, resourceHref, "write")
}

// ParsePrivilegeSet converts a slice of privilege strings to a PrivilegeSet struct.
func ParsePrivilegeSet(privileges []string) PrivilegeSet {
	privSet := PrivilegeSet{}

	for _, priv := range privileges {
		setPrivilege(&privSet, strings.ToLower(priv))
	}

	return privSet
}

func setPrivilege(privSet *PrivilegeSet, priv string) {
	privilegeSetters := map[string]func(*PrivilegeSet){
		"read":                            func(p *PrivilegeSet) { p.Read = true },
		"write":                           func(p *PrivilegeSet) { p.Write = true },
		"write-properties":                func(p *PrivilegeSet) { p.WriteProperties = true },
		"write-content":                   func(p *PrivilegeSet) { p.WriteContent = true },
		"read-current-user-privilege-set": func(p *PrivilegeSet) { p.ReadCurrentUserPrivilegeSet = true },
		"read-acl":                        func(p *PrivilegeSet) { p.ReadACL = true },
		"write-acl":                       func(p *PrivilegeSet) { p.WriteACL = true },
		"all":                             func(p *PrivilegeSet) { p.All = true },
		"calendar-access":                 func(p *PrivilegeSet) { p.CalendarAccess = true },
		"read-free-busy":                  func(p *PrivilegeSet) { p.ReadFreeBusy = true },
		"schedule-inbox":                  func(p *PrivilegeSet) { p.ScheduleInbox = true },
		"schedule-outbox":                 func(p *PrivilegeSet) { p.ScheduleOutbox = true },
		"schedule-send":                   func(p *PrivilegeSet) { p.ScheduleSend = true },
		"schedule-deliver":                func(p *PrivilegeSet) { p.ScheduleDeliver = true },
	}

	if setter, ok := privilegeSetters[priv]; ok {
		setter(privSet)
	}
}

// ToStringSlice converts a PrivilegeSet to a slice of privilege strings.
func (ps PrivilegeSet) ToStringSlice() []string {
	var privileges []string

	if ps.Read {
		privileges = append(privileges, "read")
	}
	if ps.Write {
		privileges = append(privileges, "write")
	}
	if ps.WriteProperties {
		privileges = append(privileges, "write-properties")
	}
	if ps.WriteContent {
		privileges = append(privileges, "write-content")
	}
	if ps.ReadCurrentUserPrivilegeSet {
		privileges = append(privileges, "read-current-user-privilege-set")
	}
	if ps.ReadACL {
		privileges = append(privileges, "read-acl")
	}
	if ps.WriteACL {
		privileges = append(privileges, "write-acl")
	}
	if ps.All {
		privileges = append(privileges, "all")
	}
	if ps.CalendarAccess {
		privileges = append(privileges, "calendar-access")
	}
	if ps.ReadFreeBusy {
		privileges = append(privileges, "read-free-busy")
	}
	if ps.ScheduleInbox {
		privileges = append(privileges, "schedule-inbox")
	}
	if ps.ScheduleOutbox {
		privileges = append(privileges, "schedule-outbox")
	}
	if ps.ScheduleSend {
		privileges = append(privileges, "schedule-send")
	}
	if ps.ScheduleDeliver {
		privileges = append(privileges, "schedule-deliver")
	}

	return privileges
}
