package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	caldav "github.com/kevmarchant/go-icloud-caldav"
)

func main() {
	username := os.Getenv("ICLOUD_USERNAME")
	password := os.Getenv("ICLOUD_PASSWORD")

	if username == "" || password == "" {
		log.Fatal("Please set ICLOUD_USERNAME and ICLOUD_PASSWORD environment variables")
	}

	client := caldav.NewClient(username, password)

	fmt.Println("Discovering calendars...")
	ctx := context.Background()
	calendars, err := client.DiscoverCalendars(ctx)
	if err != nil {
		log.Fatalf("Failed to discover calendars: %v", err)
	}

	fmt.Printf("Found %d calendars:\n", len(calendars))
	for _, cal := range calendars {
		fmt.Printf("- %s (%s)\n", cal.DisplayName, cal.Name)
		if cal.Description != "" {
			fmt.Printf("  Description: %s\n", cal.Description)
		}
	}

	if len(calendars) == 0 {
		fmt.Println("No calendars found")
		return
	}

	firstCalendar := calendars[0]
	fmt.Printf("\nGetting recent events from '%s'...\n", firstCalendar.DisplayName)

	events, err := client.GetRecentEvents(ctx, firstCalendar.Href, 7)
	if err != nil {
		log.Printf("Failed to get recent events: %v", err)
		return
	}

	fmt.Printf("Found %d events in the last/next 7 days:\n", len(events))
	for _, event := range events {
		fmt.Printf("- %s\n", event.Summary)
		if event.StartTime != nil {
			fmt.Printf("  Start: %s\n", event.StartTime.Format(time.RFC3339))
		}
		if event.Location != "" {
			fmt.Printf("  Location: %s\n", event.Location)
		}
	}

	fmt.Printf("\nCounting total events in '%s'...\n", firstCalendar.DisplayName)
	count, err := client.CountEvents(ctx, firstCalendar.Href)
	if err != nil {
		log.Printf("Failed to count events: %v", err)
		return
	}
	fmt.Printf("Total events: %d\n", count)

	now := time.Now()
	start := now.AddDate(0, 0, -30)
	end := now.AddDate(0, 0, 30)

	fmt.Printf("\nGetting events between %s and %s...\n",
		start.Format("2006-01-02"),
		end.Format("2006-01-02"))

	rangedEvents, err := client.GetEventsByTimeRange(ctx, firstCalendar.Href, start, end)
	if err != nil {
		log.Printf("Failed to get events by time range: %v", err)
		return
	}

	fmt.Printf("Found %d events in the specified time range\n", len(rangedEvents))

	fmt.Println("\nSearching for events containing 'meeting'...")
	searchResults, err := client.SearchEvents(ctx, firstCalendar.Href, "meeting")
	if err != nil {
		log.Printf("Failed to search events: %v", err)
		return
	}

	fmt.Printf("Found %d events matching 'meeting':\n", len(searchResults))
	for _, event := range searchResults {
		fmt.Printf("- %s\n", event.Summary)
	}

	fmt.Println("\nExample complete!")
}
