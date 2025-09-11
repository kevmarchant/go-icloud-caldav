package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	caldav "github.com/kevmarchant/go-icloud-caldav"
)

func main() {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("Please set ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables")
	}

	fmt.Printf("Testing with username: %s\n", username)

	client := caldav.NewClient(username, password)

	fmt.Println("\n1. Finding current user principal...")
	ctx := context.Background()
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

	fmt.Println("\n3. Finding calendars...")
	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil {
		log.Printf("Error finding calendars: %v", err)
		return
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

	if len(calendars) > 0 {
		var testCal caldav.Calendar
		for _, cal := range calendars {
			if cal.DisplayName == "Work" || cal.DisplayName == "Personal" || cal.DisplayName == "Home" {
				testCal = cal
				break
			}
		}
		if testCal.DisplayName == "" {
			testCal = calendars[0]
		}

		fmt.Printf("\n4. Getting recent events from '%s'...\n", testCal.DisplayName)

		events, err := client.GetRecentEvents(ctx, testCal.Href, 7)
		if err != nil {
			log.Printf("Error getting events: %v", err)
		} else {
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

		fmt.Printf("\n5. Counting total events in '%s'...\n", testCal.DisplayName)
		count, err := client.CountEvents(ctx, testCal.Href)
		if err != nil {
			log.Printf("Error counting events: %v", err)
		} else {
			fmt.Printf("   Total events: %d\n", count)
		}

		fmt.Println("\n6. Searching for 'Meeting' event...")
		foundTargetEvent := false
		for _, cal := range calendars {
			events, err := client.GetAllEvents(ctx, cal.Href)
			if err == nil {
				for _, event := range events {
					summaryLower := strings.ToLower(event.Summary)
					if strings.Contains(summaryLower, "meeting") {
						foundTargetEvent = true
						fmt.Printf("   Found in calendar '%s'!\n", cal.DisplayName)
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
						break
					}
				}
			}
			if foundTargetEvent {
				break
			}
		}
		if !foundTargetEvent {
			fmt.Println("   'Meeting' event not found")
		}

		fmt.Println("\n7. Getting all events from Personal calendar...")
		for _, cal := range calendars {
			if cal.DisplayName == "Personal" {
				events, err := client.GetAllEvents(ctx, cal.Href)
				if err != nil {
					fmt.Printf("   Error: %v\n", err)
				} else {
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
				}
				break
			}
		}
	}

	fmt.Println("\nTest complete!")
}
