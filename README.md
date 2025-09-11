# go-icloud-caldav

A pure Go CalDAV client library specifically designed for iCloud compatibility. This library addresses the XML generation issues found in other CalDAV libraries when working with iCloud's CalDAV implementation.

[![Go Reference](https://pkg.go.dev/badge/github.com/kevmarchant/go-icloud-caldav.svg)](https://pkg.go.dev/github.com/kevmarchant/go-icloud-caldav)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevmarchant/go-icloud-caldav)](https://goreportcard.com/report/github.com/kevmarchant/go-icloud-caldav)
[![Test Coverage](https://img.shields.io/badge/coverage-85.6%25-brightgreen.svg)](https://github.com/kevmarchant/go-icloud-caldav)

## Features

- ✅ **iCloud CalDAV Compatibility** - Properly handles iCloud's specific CalDAV requirements
- ✅ **Zero Dependencies** - Pure Go implementation with no external dependencies
- ✅ **Proper XML Namespace Handling** - Generates correct CalDAV XML with proper namespace declarations
- ✅ **Multi-Status Response Parsing** - Correctly handles 207 Multi-Status responses
- ✅ **Calendar Discovery** - Find all calendars in an account
- ✅ **Event Retrieval** - Query events by time range, UID, or search text
- ✅ **Robust Error Handling** - Clear error messages for debugging

## Installation

```bash
go get github.com/kevmarchant/go-icloud-caldav
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    caldav "github.com/kevmarchant/go-icloud-caldav"
)

func main() {
    // Create client with iCloud credentials
    // Note: Use an app-specific password, not your regular iCloud password
    client := caldav.NewClient("user@icloud.com", "app-specific-password")
    
    // Discover all calendars
    calendars, err := client.DiscoverCalendars()
    if err != nil {
        log.Fatal(err)
    }
    
    for _, cal := range calendars {
        fmt.Printf("Calendar: %s (%s)\n", cal.DisplayName, cal.Path)
        
        // Get recent events from each calendar
        events, err := client.GetRecentEvents(cal.Path, 30)
        if err != nil {
            log.Printf("Error getting events: %v", err)
            continue
        }
        
        for _, event := range events {
            fmt.Printf("  - %s at %s\n", event.Summary, event.StartTime)
        }
    }
}
```

## Authentication

iCloud requires app-specific passwords for third-party applications:

1. Sign in to [appleid.apple.com](https://appleid.apple.com)
2. In the Sign-In and Security section, select App-Specific Passwords
3. Click the + button to generate a new password
4. Name it (e.g., "CalDAV Client") and copy the generated password
5. Use this password with your iCloud email address

## API Documentation

### Client Creation

```go
// Create a new CalDAV client
client := caldav.NewClient(username, password)

// Set custom timeout (default is 30 seconds)
client.SetTimeout(60 * time.Second)
```

### Calendar Discovery

```go
// Full discovery flow (principal → home set → calendars)
calendars, err := client.DiscoverCalendars()

// Or step by step:
principal, err := client.FindCurrentUserPrincipal()
homeSet, err := client.FindCalendarHomeSet(principal)
calendars, err := client.FindCalendars(homeSet)
```

### Event Queries

```go
// Get events from the last N days
events, err := client.GetRecentEvents(calendarPath, 7)

// Get events in a specific time range
start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
end := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)
events, err := client.GetEventsByTimeRange(calendarPath, start, end)

// Get a specific event by UID
event, err := client.GetEventByUID(calendarPath, "event-uid-123")

// Search for events by text
events, err := client.SearchEvents(calendarPath, "meeting")

// Get all events (within a 4-year window)
events, err := client.GetAllEvents(calendarPath)

// Count events in a calendar
count, err := client.CountEvents(calendarPath)
```

### Advanced Queries

```go
// Custom CalDAV query
query := caldav.CalendarQuery{
    Properties: []string{"getetag", "calendar-data"},
    Filter: caldav.Filter{
        Component: "VEVENT",
        TimeRange: &caldav.TimeRange{
            Start: time.Now().AddDate(0, -1, 0),
            End:   time.Now().AddDate(0, 1, 0),
        },
        Properties: []caldav.PropFilter{
            {
                Name: "SUMMARY",
                TextMatch: &caldav.TextMatch{
                    Value: "team meeting",
                },
            },
        },
    },
}
events, err := client.QueryCalendar(calendarPath, query)
```

## Data Structures

### Calendar

```go
type Calendar struct {
    Path                  string
    DisplayName           string
    Description           string
    Color                 string
    Order                 int
    SupportedComponents   []string
    ResourceType          []string
    CTag                  string
    ETag                  string
}
```

### CalendarObject (Event)

```go
type CalendarObject struct {
    Path        string
    ETag        string
    Data        string      // Raw iCalendar data
    UID         string
    Summary     string
    Description string
    Location    string
    StartTime   *time.Time
    EndTime     *time.Time
    Organizer   string
    Attendees   []string
}
```

## Error Handling

The library provides detailed error messages for common issues:

```go
events, err := client.GetRecentEvents(calendarPath, 30)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "401"):
        // Authentication failed - check credentials
    case strings.Contains(err.Error(), "404"):
        // Calendar not found - check path
    case strings.Contains(err.Error(), "412"):
        // Precondition failed - usually a query issue
    default:
        // Other error
    }
}
```

## Testing

Run the test suite:

```bash
# Run unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests (requires environment variables)
ICLOUD_USERNAME="user@icloud.com" \
ICLOUD_PASSWORD="app-specific-password" \
go test -tags=integration ./...
```

## Why This Library?

### The Problem

Popular Go CalDAV libraries generate incorrect XML for CalDAV REPORT requests when working with iCloud:

**Incorrect XML** (causes 404 errors with iCloud):

```xml
<prop name="UID">
```

**Correct XML** (what this library generates):

```xml
<C:prop-filter name="UID">
  <C:text-match>some-uid</C:text-match>
</C:prop-filter>
```

### The Solution

This library:

- Generates proper CalDAV XML with correct namespace handling
- Handles iCloud's specific requirements (time ranges, proper filters)
- Provides a clean, simple API focused on common use cases
- Has zero external dependencies for security and simplicity

## Limitations

This is a **read-only** CalDAV client. It can:

- ✅ Authenticate with CalDAV servers
- ✅ Discover calendars
- ✅ Retrieve events
- ✅ Search and filter events

It cannot (yet):

- ❌ Create new events
- ❌ Update existing events
- ❌ Delete events
- ❌ Create or modify calendars

Write operations may be added in a future release based on demand.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

1. Clone the repository
2. Install Go 1.21 or later
3. Run tests: `go test ./...`
4. Format code: `go fmt ./...`
5. Check for issues: `go vet ./...`

## License

MIT License - see [LICENSE](LICENSE) file for details

## Acknowledgments

This library was created to solve real-world issues with iCloud CalDAV integration. Special thanks to the CalDAV specification authors and the Go community.

## Support

For issues, questions, or contributions, please use the [GitHub issue tracker](https://github.com/kevmarchant/go-icloud-caldav/issues).
