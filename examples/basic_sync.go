//go:build ignore
// +build ignore

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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Println("🔍 Discovering calendars...")
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		log.Fatalf("Failed to find user principal: %v", err)
	}

	homeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		log.Fatalf("Failed to find calendar home: %v", err)
	}

	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil {
		log.Fatalf("Failed to find calendars: %v", err)
	}

	fmt.Printf("✅ Found %d calendars\n\n", len(calendars))

	totalEvents := 0
	for _, calendar := range calendars {
		fmt.Printf("📅 Calendar: %s\n", calendar.DisplayName)
		fmt.Printf("   Path: %s\n", calendar.Href)
		fmt.Printf("   Supported Components: %v\n", calendar.SupportedComponents)

		fmt.Println("   Fetching recent events...")
		events, err := client.GetRecentEvents(ctx, calendar.Href, 30)
		if err != nil {
			if caldav.IsNotFound(err) {
				fmt.Println("   ⚠️  Calendar not accessible")
				continue
			}
			fmt.Printf("   ❌ Error fetching events: %v\n", err)
			continue
		}

		fmt.Printf("   📊 Found %d events in the last 30 days\n", len(events))
		totalEvents += len(events)

		if len(events) > 0 && len(events) <= 5 {
			fmt.Println("   Sample events:")
			for i, event := range events {
				if i >= 3 {
					break
				}
				fmt.Printf("     - %s\n", extractSummary(event.CalendarData))
			}
		}

		fmt.Println()
	}

	fmt.Printf("📈 Total events across all calendars: %d\n", totalEvents)

	fmt.Println("\n🔄 Fetching events for a specific time range...")
	startTime := time.Now().AddDate(0, 0, -7)
	endTime := time.Now().AddDate(0, 0, 7)

	if len(calendars) > 0 {
		calendar := calendars[0]

		events, err := client.GetEventsByTimeRange(
			ctx,
			calendar.Href,
			startTime,
			endTime,
		)
		if err != nil {
			log.Printf("Failed to fetch events by time range: %v", err)
		} else {
			fmt.Printf("📅 Found %d events in %s between %s and %s\n",
				len(events),
				calendar.DisplayName,
				startTime.Format("2006-01-02"),
				endTime.Format("2006-01-02"),
			)
		}
	}

	fmt.Println("\n✨ Basic sync completed successfully!")
}

func extractSummary(icalData string) string {
	lines := []string{}
	for _, line := range []byte(icalData) {
		if line == '\n' {
			lines = append(lines, string([]byte{}))
		} else if len(lines) == 0 {
			lines = append(lines, string(line))
		} else {
			lines[len(lines)-1] += string(line)
		}
	}

	for _, line := range lines {
		if len(line) > 8 && line[:8] == "SUMMARY:" {
			return line[8:]
		}
	}

	return "(No summary)"
}
