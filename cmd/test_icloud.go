package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	caldav "github.com/kevmarchant/go-icloud-caldav"
)

func validateCredentials() (string, string) {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("Please set ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables")
	}

	fmt.Printf("Testing with username: %s\n", username)
	return username, password
}

func findPrincipalAndHomeSet(ctx context.Context, client *caldav.CalDAVClient) (string, string) {
	fmt.Println("\n1. Finding current user principal...")
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Printf("Error finding principal: %v", err)
	} else {
		fmt.Printf("   Principal: %s\n", principal)
	}

	fmt.Println("\n2. Finding calendar home set...")
	homeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Printf("Error finding home set: %v", err)
	} else {
		fmt.Printf("   Home set: %s\n", homeSet)
	}

	return principal, homeSet
}

func discoverCalendars(ctx context.Context, client *caldav.CalDAVClient, homeSet string) []caldav.Calendar {
	fmt.Println("\n3. Finding calendars...")
	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil {
		log.Printf("Error finding calendars: %v", err)
		return nil
	}

	fmt.Printf("   Found %d calendars:\n", len(calendars))
	for i, cal := range calendars {
		fmt.Printf("   %d. %s (%s)\n", i+1, cal.DisplayName, cal.Href)
		if cal.Description != "" {
			fmt.Printf("      Description: %s\n", cal.Description)
		}
		if cal.Color != "" {
			fmt.Printf("      Color: %s\n", cal.Color)
		}
	}
	return calendars
}

func selectTestCalendar(calendars []caldav.Calendar) caldav.Calendar {
	preferredNames := []string{"Work", "Personal", "Home"}
	for _, cal := range calendars {
		for _, name := range preferredNames {
			if cal.DisplayName == name {
				return cal
			}
		}
	}
	if len(calendars) > 0 {
		return calendars[0]
	}
	return caldav.Calendar{}
}

func testRecentEvents(ctx context.Context, client *caldav.CalDAVClient, testCal caldav.Calendar) {
	fmt.Printf("\n4. Getting recent events from '%s'...\n", testCal.DisplayName)

	events, err := client.GetRecentEvents(ctx, testCal.Href, 7)
	if err != nil {
		log.Printf("Error getting events: %v", err)
		return
	}

	fmt.Printf("   Found %d events\n", len(events))
	for i, event := range events {
		if i >= 5 {
			fmt.Println("   ... (showing first 5)")
			break
		}
		fmt.Printf("   - %s\n", event.Summary)
		if event.Location != "" {
			fmt.Printf("     Location: %s\n", event.Location)
		}
	}
}

func testEventCount(ctx context.Context, client *caldav.CalDAVClient, testCal caldav.Calendar) {
	fmt.Printf("\n5. Counting total events in '%s'...\n", testCal.DisplayName)
	count, err := client.CountEvents(ctx, testCal.Href)
	if err != nil {
		log.Printf("Error counting events: %v", err)
	} else {
		fmt.Printf("   Total events: %d\n", count)
	}
}

func printEventDetails(event caldav.CalendarObject, calName string) {
	fmt.Printf("   Found in calendar '%s'!\n", calName)
	fmt.Printf("   - Summary: %s\n", event.Summary)
	if event.Location != "" {
		fmt.Printf("   - Location: %s\n", event.Location)
	}
	if event.StartTime != nil {
		fmt.Printf("   - Start: %v\n", event.StartTime)
	}
	if event.Description != "" {
		fmt.Printf("   - Description: %s\n", event.Description)
	}
}

func searchForMeetingEvent(ctx context.Context, client *caldav.CalDAVClient, calendars []caldav.Calendar) {
	fmt.Println("\n6. Searching for 'Meeting' event...")

	for _, cal := range calendars {
		events, err := client.GetAllEvents(ctx, cal.Href)
		if err != nil {
			continue
		}

		for _, event := range events {
			summaryLower := strings.ToLower(event.Summary)
			if strings.Contains(summaryLower, "meeting") {
				printEventDetails(event, cal.DisplayName)
				return
			}
		}
	}

	fmt.Println("   'Meeting' event not found")
}

func testPersonalCalendar(ctx context.Context, client *caldav.CalDAVClient, calendars []caldav.Calendar) {
	fmt.Println("\n7. Getting all events from Personal calendar...")

	for _, cal := range calendars {
		if cal.DisplayName != "Personal" {
			continue
		}

		events, err := client.GetAllEvents(ctx, cal.Href)
		if err != nil {
			fmt.Printf("   Error: %v\n", err)
			return
		}

		fmt.Printf("   Found %d events in Personal calendar\n", len(events))
		for i, event := range events {
			if i >= 10 {
				fmt.Println("   ... (showing first 10)")
				break
			}
			fmt.Printf("   - %s", event.Summary)
			if event.StartTime != nil {
				fmt.Printf(" (%v)", event.StartTime.Format("Jan 2, 2006"))
			}
			fmt.Println()
		}
		return
	}
}

func main() {
	username, password := validateCredentials()
	client := caldav.NewClient(username, password)
	ctx := context.Background()

	principal, homeSet := findPrincipalAndHomeSet(ctx, client)
	_ = principal

	calendars := discoverCalendars(ctx, client, homeSet)
	if len(calendars) == 0 {
		fmt.Println("No calendars found, exiting...")
		return
	}

	testCal := selectTestCalendar(calendars)
	if testCal.DisplayName != "" {
		testRecentEvents(ctx, client, testCal)
		testEventCount(ctx, client, testCal)
	}

	searchForMeetingEvent(ctx, client, calendars)
	testPersonalCalendar(ctx, client, calendars)

	fmt.Println("\nTest complete!")
}
