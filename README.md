# go-icloud-caldav

A comprehensive CalDAV client library for Go with full iCloud compatibility, advanced filtering, parallel operations, and incremental sync support.

[![Go Reference](https://pkg.go.dev/badge/github.com/kevmarchant/go-icloud-caldav.svg)](https://pkg.go.dev/github.com/kevmarchant/go-icloud-caldav)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevmarchant/go-icloud-caldav)](https://goreportcard.com/report/github.com/kevmarchant/go-icloud-caldav)
[![Test Coverage](https://img.shields.io/badge/coverage-83.6%25-brightgreen.svg)](https://github.com/kevmarchant/go-icloud-caldav)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Authentication](#authentication)
- [API Documentation](#api-documentation)
  - [Client Creation](#client-creation)
  - [Calendar Discovery](#calendar-discovery)
  - [Event Queries](#event-queries)
  - [Parallel Operations](#parallel-operations)
  - [Advanced Filtering](#advanced-filtering)
  - [Incremental Sync](#incremental-sync-rfc-6578)
  - [Error Handling](#error-handling)
  - [Connection Management](#connection-management)
  - [XML Validation](#xml-validation)
- [Data Structures](#data-structures)
- [Examples](#examples)
- [Performance](#performance)
- [Testing](#testing)
- [Development](#development)
- [Contributing](#contributing)
- [Why This Library?](#why-this-library)
- [Limitations](#limitations)
- [Changelog](#changelog)
- [License](#license)
- [Support](#support)

## Features

### Core Features

- ✅ **iCloud CalDAV Compatibility** - Properly handles iCloud's specific CalDAV requirements
- ✅ **Zero Dependencies** - Pure Go implementation with no external dependencies
- ✅ **Proper XML Namespace Handling** - Generates correct CalDAV XML with proper namespace declarations
- ✅ **Multi-Status Response Parsing** - Correctly handles 207 Multi-Status responses

### Advanced Features

- 🚀 **Parallel Operations** - Query multiple calendars concurrently with up to 10x speedup
- 📊 **Built-in iCal Parser** - Automatically parse calendar data into structured Go types
- 🔄 **Incremental Sync** - Support for sync tokens (RFC 6578) for efficient updates
- 🛡️ **XML Validation** - Auto-correct common XML issues before sending requests
- 🔌 **Connection Management** - Connection pooling, retry logic with exponential backoff
- 🎯 **Typed Errors** - Comprehensive error categorisation for better error handling
- 🔍 **Advanced Filtering** - Nested component filters, text matching, time ranges
- 📈 **Metrics Collection** - Track connection reuse, retry attempts, and performance

### New in v0.2.1

- 🗄️ **Response Caching** - Intelligent caching with TTL and statistics
- 🔄 **RRULE Parsing** - Native recurrence rule parsing and expansion
- 🌍 **Enhanced Timezone Support** - Better timezone handling and conversion
- 🔐 **Access Control Lists** - Calendar permission management
- 📎 **Attachment Support** - Handle calendar event attachments
- 🎯 **Server Compatibility** - Automatic detection of CalDAV server types (iCloud, Google, Nextcloud)

## Installation

```bash
go get github.com/kevmarchant/go-icloud-caldav
```

Requirements:

- Go 1.21 or later
- iCloud account with app-specific password

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/kevmarchant/go-icloud-caldav"
)

func main() {
    // Create client with iCloud credentials
    // Note: Use an app-specific password, not your regular iCloud password
    client, err := caldav.NewClient("user@icloud.com", "app-specific-password",
        caldav.WithAutoParsing(),  // Enable automatic iCal parsing
        caldav.WithTimeout(2*time.Minute),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // Discover all calendars
    calendars, err := client.FindCalendars(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    // Parallel sync across all calendars
    results := client.GetRecentEventsParallel(ctx, calendars, 30, 5)
    
    for _, result := range results {
        if result.Error != nil {
            log.Printf("Error in %s: %v", result.CalendarPath, result.Error)
            continue
        }
        
        fmt.Printf("Calendar: %s - %d events\n", result.CalendarPath, len(result.Objects))
        for _, obj := range result.Objects {
            if obj.ParsedData != nil && len(obj.ParsedData.Events) > 0 {
                event := obj.ParsedData.Events[0]
                fmt.Printf("  - %s at %s\n", event.Summary, event.DateTimeStart)
            }
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
// Basic client
client, err := caldav.NewClient(username, password)

// Client with options
client, err := caldav.NewClient(username, password,
    caldav.WithTimeout(2*time.Minute),
    caldav.WithAutoParsing(),                    // Parse iCal data automatically
    caldav.WithDebugLogging(true),               // Enable debug logs
    caldav.WithXMLValidation(true, true),        // Validate and auto-correct XML
    caldav.WithRetry(retryConfig),               // Enable retry logic
    caldav.WithConnectionPool(poolConfig),       // Configure connection pooling
    caldav.WithConnectionMetrics(metrics),       // Enable metrics collection
)
```

### Calendar Discovery

```go
ctx := context.Background()

// Discover all calendars
calendars, err := client.FindCalendars(ctx)

// Step by step discovery:
principal, err := client.FindCurrentUserPrincipal(ctx)
homeSet, err := client.FindCalendarHomeSet(ctx, principal)
calendars, err := client.ListCalendars(ctx, homeSet)
```

### Event Queries

```go
// Get events from the last N days
events, err := client.GetRecentEvents(ctx, calendarPath, 7)

// Get events in a specific time range
events, err := client.GetEventsByTimeRange(ctx, calendarPath, start, end)

// Get upcoming events
events, err := client.GetUpcomingEvents(ctx, calendarPath, 90)

// Get all events (within a 4-year window)
events, err := client.GetAllEvents(ctx, calendarPath)

// Count events in a calendar
count, err := client.CountEvents(ctx, calendarPath)
```

### Parallel Operations

```go
// Query multiple calendars in parallel
results := client.QueryCalendarsParallel(ctx, calendars, query, 10)

// Get recent events from all calendars in parallel
results := client.GetRecentEventsParallel(ctx, calendars, 30, 10)

// Get events by time range in parallel
results := client.GetEventsByTimeRangeParallel(ctx, calendars, start, end, 5)

// Helper functions for results
successful := caldav.FilterSuccessfulResults(results)
failed := caldav.FilterFailedResults(results)
allEvents := caldav.AggregateResults(results)
totalCount := caldav.CountObjectsInResults(results)
```

### Advanced Filtering

```go
// Complex nested filter
query := caldav.CalendarQuery{
    Filter: caldav.Filter{
        Component: "VEVENT",
        TimeRange: &caldav.TimeRange{
            Start: time.Now(),
            End:   time.Now().AddDate(0, 1, 0),
        },
        Props: []caldav.PropFilter{
            {
                Name: "STATUS",
                TextMatch: &caldav.TextMatch{
                    Text:            "TENTATIVE",
                    Collation:       "i;unicode-casemap",
                    NegateCondition: false,
                },
            },
        },
        CompFilters: []caldav.Filter{
            {
                Component: "VALARM",  // Find events with alarms
            },
        },
    },
    Props: []string{"SUMMARY", "DTSTART", "STATUS"},
}

events, err := client.QueryCalendar(ctx, calendarPath, query)
```

### Incremental Sync (RFC 6578)

```go
// Initial sync to get all events and sync token
response, err := client.InitialSync(ctx, calendarPath)
syncToken := response.SyncToken

// Later: incremental sync with token
response, err := client.IncrementalSync(ctx, calendarPath, syncToken)

// Process changes
newItems := response.GetNewItems()
modifiedItems := response.GetModifiedItems()
deletedItems := response.GetDeletedItems()

// Sync all calendars
results := client.SyncAllCalendars(ctx, calendars, tokenMap)
```

### Error Handling

```go
events, err := client.GetRecentEvents(ctx, calendarPath, 30)
if err != nil {
    // Check error type
    if caldav.IsAuthError(err) {
        // Handle authentication error
    } else if caldav.IsNotFound(err) {
        // Handle not found error
    } else if caldav.IsNetworkError(err) {
        // Handle network error
    } else if caldav.IsTemporary(err) {
        // Retry on temporary errors
    }
    
    // Get detailed error information
    var calErr *caldav.CalDAVError
    if errors.As(err, &calErr) {
        fmt.Printf("Type: %v, Code: %d, Message: %s\n",
            calErr.Type, calErr.StatusCode, calErr.Message)
    }
}
```

### Connection Management

```go
// Configure connection pooling
poolConfig := caldav.ConnectionPoolConfig{
    MaxIdleConns:        10,
    MaxIdleConnsPerHost: 5,
    MaxConnsPerHost:     10,
    IdleConnTimeout:     90 * time.Second,
}

// Configure retry logic
retryConfig := caldav.RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 1 * time.Second,
    MaxBackoff:     10 * time.Second,
    BackoffFactor:  2.0,
    UseJitter:      true,
}

// Enable metrics
metrics := &caldav.ConnectionMetrics{}

client, err := caldav.NewClient(username, password,
    caldav.WithConnectionPool(poolConfig),
    caldav.WithRetry(retryConfig),
    caldav.WithConnectionMetrics(metrics),
)

// Check metrics
fmt.Printf("Requests: %d, Retries: %d, Reused: %d\n",
    metrics.TotalRequests,
    metrics.RetriedRequests,
    metrics.ReusedConnections)
```

### XML Validation

The library includes comprehensive XML validation and auto-correction capabilities to ensure CalDAV requests are properly formatted.

#### Basic Usage

```go
// Enable XML validation with auto-correction
client, err := caldav.NewClient(username, password,
    caldav.WithAutoCorrectXML())

// Enable strict XML validation (no auto-correction)
client, err := caldav.NewClient(username, password,
    caldav.WithStrictXMLValidation())

// Custom validation settings
client, err := caldav.NewClient(username, password,
    caldav.WithXMLValidation(true, false))  // autoCorrect=true, strictMode=false
```

#### Validation Rules

The XML validator checks for:

1. **Well-Formedness** - Proper XML structure with matching tags
2. **Namespace Validation** - Required DAV: and CalDAV namespaces
3. **CalDAV Structure** - VCALENDAR root requirement, component validation
4. **Time Format** - Correct CalDAV time format (YYYYMMDDTHHMMSSZ)
5. **Property Filters** - Proper prop-filter element structure

#### Auto-Correction Features

When enabled, the validator automatically fixes:

- **Double VCALENDAR nesting** - Removes redundant VCALENDAR wrappers
- **Incorrect prop-filter syntax** - Converts to proper CalDAV format
- **Time format issues** - Converts ISO 8601 to CalDAV format
- **Missing namespaces** - Adds required namespace declarations

#### Example Corrections

```xml
<!-- Before: Double VCALENDAR -->
<C:comp-filter name="VCALENDAR">
  <C:comp-filter name="VCALENDAR">
    <C:comp-filter name="VEVENT"/>
  </C:comp-filter>
</C:comp-filter>

<!-- After: Fixed -->
<C:comp-filter name="VCALENDAR">
  <C:comp-filter name="VEVENT"/>
</C:comp-filter>
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
    SupportedComponentSet []string
    ResourceType          []string
    CTag                  string
    ETag                  string
}
```

### CalendarObject

```go
type CalendarObject struct {
    Path        string
    ETag        string
    Data        string                 // Raw iCalendar data
    ParsedData  *ParsedCalendarData    // Parsed data (when WithAutoParsing enabled)
    ParseError  error                  // Parsed error, if any
}
```

### ParsedCalendarData

```go
type ParsedCalendarData struct {
    ProdID       string
    Version      string
    CalScale     string
    Method       string
    Events       []Event
    Todos        []Todo
    Journals     []Journal
    FreeBusy     []FreeBusy
    TimeZones    []TimeZone
    CustomProps  map[string]string
}
```

### Event

```go
type Event struct {
    UID              string
    DateTimeStamp    time.Time
    DateTimeStart    time.Time
    DateTimeEnd      time.Time
    Summary          string
    Description      string
    Location         string
    Status           string
    Organizer        *Organizer
    Attendees        []Attendee
    Alarms           []Alarm
    RecurrenceRule   string
    Categories       []string
    Priority         int
    Sequence         int
    CustomProps      map[string]string
}
```

## Examples

The `examples/` directory contains comprehensive examples demonstrating all features:

- **[basic_sync.go](examples/basic_sync.go)** - Simple calendar discovery and event fetching
- **[parallel_sync.go](examples/parallel_sync.go)** - Parallel operations with performance comparison
- **[incremental_sync.go](examples/incremental_sync.go)** - Incremental sync using sync tokens
- **[error_handling.go](examples/error_handling.go)** - Error handling patterns and retry logic
- **[filtering.go](examples/filtering.go)** - Advanced filtering and XML validation

Run examples:

```bash
# Set credentials
export ICLOUD_USERNAME="user@icloud.com"
export ICLOUD_PASSWORD="app-specific-password"

# Run examples
go run examples/basic_sync.go
go run examples/parallel_sync.go
go run examples/incremental_sync.go
go run examples/error_handling.go
go run examples/filtering.go
```

## Performance

The library includes several performance optimisations:

### Using Parallel Operations

- **10x speedup** with 10 workers for multi-calendar sync
- **5x speedup** with 5 workers (recommended for most use cases)
- Configurable worker pool size based on your needs

### Connection Pooling

- Reuses HTTP connections for multiple requests
- Reduces TLS handshake overhead
- Configurable pool size and timeout settings

### Benchmarks

```bash
# Run benchmarks
go test -bench=. -benchmem ./...

# Example results (10 calendars, 100 events each):
# BenchmarkSerialQuery-8         1    1053ms/op
# BenchmarkParallel5Query-8      1     211ms/op  (5x speedup)
# BenchmarkParallel10Query-8     1     113ms/op  (9.3x speedup)
```

Performance results from production testing:

- Successfully synced 4,182 events across 9 calendars
- 83.6% test coverage with comprehensive benchmarks
- Memory-efficient with minimal allocations

## Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with detailed coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run integration tests (requires credentials)
ICLOUD_USERNAME="user@icloud.com" \
ICLOUD_PASSWORD="app-specific-password" \
go test -tags=integration ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run with race detection
go test -race ./...
```

## Development

### Project Structure

```
go-icloud-caldav/
├── client.go            # Main CalDAVClient implementation
├── types.go             # Data structures
├── xml_builder.go       # XML request generation
├── parser.go            # Response parsing
├── calendar.go          # Calendar operations
├── events.go            # Event query operations
├── errors.go            # Typed error system
├── batch.go             # Parallel operations
├── connection.go        # Connection management
├── sync.go              # Sync token support
├── ical_parser.go       # iCal parsing
├── examples/            # Example applications
└── *_test.go            # Test files
```

### Quality Checks

Before committing code, run:

```bash
# Format code
go fmt ./...

# Vet for issues
go vet ./...

# Static analysis
staticcheck ./...

# Check for unchecked errors
errcheck ./...

# Check for ineffectual assignments
ineffassign ./...

# Comprehensive linting
golangci-lint run ./...

# All checks in one command
go fmt ./... && go vet ./... && staticcheck ./... && go test -cover ./...
```

### Common Development Commands

```bash
# Install quality tools
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/kisielk/errcheck@latest
go install github.com/gordonklaus/ineffassign@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run specific test
go test -run TestFunctionName ./...

# Build and verify
go build ./...
```

## Contributing

I very much welcome contributions - please follow these guidelines:

### Code Style

- Follow standard Go conventions
- Use `go fmt` for formatting
- Maintain test coverage above 80%
- Add tests for new features
- Update documentation for API changes

### Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add/update and run tests (`go test -cover`, `go test -tags=integration`, `staticcheck`, `errcheck`, `ineffassign`, `golangci-lint`)
5. Run quality checks (`go fmt`, `go vet`, `staticcheck`, `errcheck`, `ineffassign`, `golangci-lint`)
6. Commit with descriptive message
7. Push to your fork
8. Open a Pull Request

### Commit Message Format

```
feat: add new feature
fix: correct bug in X
docs: update README
test: add tests for Y
refactor: improve Z structure
```

### Testing Requirements

- All new features must include tests
- Bug fixes should include regression tests
- Maintain or improve test coverage
- Run tests with race detection

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
- Includes comprehensive error handling and retry logic
- Offers parallel processing for performance
- Supports incremental sync for efficiency

## Limitations

This is a **read-only** CalDAV client. It can:

- ✅ Authenticate with CalDAV servers
- ✅ Discover calendars
- ✅ Retrieve events
- ✅ Search and filter events
- ✅ Sync changes incrementally

It cannot (yet):

- ❌ Create new events
- ❌ Update existing events
- ❌ Delete events
- ❌ Create or modify calendars
- ❌ Handle recurring event modifications

Write operations may be added in a future release based on demand.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a detailed history of changes.

Recent highlights:

- v0.2.1 - Major feature expansion: caching, RRULE parsing, timezone support, ACLs, attachments
- v0.1.2 - Major feature release: parallel operations, sync tokens, iCal parser, XML validation
- v0.1.1 - Fixed CI/CD issues, improved error handling
- v0.1.0 - Initial production-ready release with full iCloud support

## License

MIT License - see [LICENSE](LICENSE) file for details

## Support

For issues, questions, or contributions:

- [GitHub Issues](https://github.com/kevmarchant/go-icloud-caldav/issues)
- [GitHub Discussions](https://github.com/kevmarchant/go-icloud-caldav/discussions)

## Request for collaboration

This library was created to allow me to sync with iCloud CalDAV integration, and current libraries had errors doing so.  If you are able to help in any way, either by suggesting improvements or contributing directly, it would be much appreciated!  Special thanks from me go to all the authors of Go and the tools that I use, in this package and others!
