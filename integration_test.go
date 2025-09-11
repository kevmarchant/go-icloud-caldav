//go:build integration
// +build integration

package caldav

import (
	"context"
	"os"
	"testing"
	"time"
)

func getTestClient(t *testing.T) *CalDAVClient {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")

	if username == "" || password == "" {
		t.Skip("ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables must be set for integration tests")
	}

	return NewClient(username, password)
}

func TestIntegrationDiscoverCalendars(t *testing.T) {
	client := getTestClient(t)

	calendars, err := client.DiscoverCalendars(context.Background())
	if err != nil {
		t.Fatalf("failed to discover calendars: %v", err)
	}

	if len(calendars) == 0 {
		t.Error("expected at least one calendar")
	}

	for _, cal := range calendars {
		t.Logf("Found calendar: %s (%s)", cal.DisplayName, cal.Href)
		if cal.Description != "" {
			t.Logf("  Description: %s", cal.Description)
		}
		if cal.Color != "" {
			t.Logf("  Color: %s", cal.Color)
		}
		if len(cal.SupportedComponents) > 0 {
			t.Logf("  Supported components: %v", cal.SupportedComponents)
		}
	}
}

func TestIntegrationFindCurrentUserPrincipal(t *testing.T) {
	client := getTestClient(t)

	principal, err := client.FindCurrentUserPrincipal(context.Background())
	if err != nil {
		t.Fatalf("failed to find current user principal: %v", err)
	}

	if principal == "" {
		t.Error("expected non-empty principal")
	}

	t.Logf("Current user principal: %s", principal)
}

func TestIntegrationFindCalendarHomeSet(t *testing.T) {
	client := getTestClient(t)

	principal, err := client.FindCurrentUserPrincipal(context.Background())
	if err != nil {
		t.Fatalf("failed to find current user principal: %v", err)
	}

	homeSet, err := client.FindCalendarHomeSet(context.Background(), principal)
	if err != nil {
		t.Fatalf("failed to find calendar home set: %v", err)
	}

	if homeSet == "" {
		t.Error("expected non-empty calendar home set")
	}

	t.Logf("Calendar home set: %s", homeSet)
}

func TestIntegrationGetRecentEvents(t *testing.T) {
	client := getTestClient(t)

	calendars, err := client.DiscoverCalendars(context.Background())
	if err != nil {
		t.Fatalf("failed to discover calendars: %v", err)
	}

	if len(calendars) == 0 {
		t.Skip("no calendars found")
	}

	for _, cal := range calendars {
		t.Logf("Getting recent events from calendar: %s", cal.DisplayName)

		events, err := client.GetRecentEvents(context.Background(), cal.Href, 30)
		if err != nil {
			t.Logf("  Error getting events: %v", err)
			continue
		}

		t.Logf("  Found %d events", len(events))
		for i, event := range events {
			if i >= 5 {
				break
			}
			t.Logf("    - %s (UID: %s)", event.Summary, event.UID)
			if event.StartTime != nil {
				t.Logf("      Start: %s", event.StartTime.Format(time.RFC3339))
			}
			if event.Location != "" {
				t.Logf("      Location: %s", event.Location)
			}
		}
	}
}

func TestIntegrationSearchForSpecificEvent(t *testing.T) {
	client := getTestClient(t)

	calendars, err := client.DiscoverCalendars(context.Background())
	if err != nil {
		t.Fatalf("failed to discover calendars: %v", err)
	}

	searchText := "Team meeting"
	foundEvent := false

	for _, cal := range calendars {
		events, err := client.SearchEvents(context.Background(), cal.Href, searchText)
		if err != nil {
			continue
		}

		for _, event := range events {
			if event.Summary == searchText {
				foundEvent = true
				t.Logf("Found event '%s' in calendar %s", searchText, cal.DisplayName)
				t.Logf("  UID: %s", event.UID)
				if event.StartTime != nil {
					t.Logf("  Start: %s", event.StartTime.Format(time.RFC3339))
				}
				if event.Location != "" {
					t.Logf("  Location: %s", event.Location)
				}
			}
		}
	}

	if !foundEvent {
		t.Logf("Event '%s' not found in any calendar", searchText)
	}
}

func TestIntegrationCountEvents(t *testing.T) {
	client := getTestClient(t)

	calendars, err := client.DiscoverCalendars(context.Background())
	if err != nil {
		t.Fatalf("failed to discover calendars: %v", err)
	}

	totalEvents := 0
	for _, cal := range calendars {
		count, err := client.CountEvents(context.Background(), cal.Href)
		if err != nil {
			t.Logf("Error counting events in %s: %v", cal.DisplayName, err)
			continue
		}

		t.Logf("Calendar %s has %d events", cal.DisplayName, count)
		totalEvents += count
	}

	t.Logf("Total events across all calendars: %d", totalEvents)
}

func TestIntegrationGetUpcomingEvents(t *testing.T) {
	client := getTestClient(t)

	calendars, err := client.DiscoverCalendars(context.Background())
	if err != nil {
		t.Fatalf("failed to discover calendars: %v", err)
	}

	if len(calendars) == 0 {
		t.Skip("no calendars found")
	}

	for _, cal := range calendars {
		events, err := client.GetUpcomingEvents(context.Background(), cal.Href, 10)
		if err != nil {
			continue
		}

		if len(events) > 0 {
			t.Logf("Upcoming events in %s:", cal.DisplayName)
			for i, event := range events {
				if i >= 5 {
					break
				}
				t.Logf("  - %s", event.Summary)
				if event.StartTime != nil {
					t.Logf("    Start: %s", event.StartTime.Format(time.RFC3339))
				}
			}
		}
	}
}
