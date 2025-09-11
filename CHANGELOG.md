# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2025-01-11

### Fixed
- CI/CD pipeline errors with golangci-lint (updated to v1.61.0 for Go 1.23 compatibility)
- Unchecked error returns in defer statements and test files
- Security scan permission issues in GitHub Actions workflow
- Code formatting consistency across all test files

### Changed
- Added proper error handling for `resp.Body.Close()` in defer statements
- Added error handling for `w.Write()` calls in test files
- Added `security-events: write` permission for Trivy security scanner
- Applied `go fmt` to ensure consistent formatting

### Technical Details
- Fixed 7 errcheck lint issues across multiple files
- Updated `.github/workflows/ci.yml` with proper permissions
- All CI/CD checks now passing

## [0.1.0] - 2025-01-11

### Initial Release

First production-ready release of go-icloud-caldav, a pure Go CalDAV client library specifically designed for iCloud compatibility.

### Core Features

#### Calendar Operations
- **FindCurrentUserPrincipal** - Discovers user's principal URL
- **FindCalendarHomeSet** - Finds calendar home collection
- **FindCalendars** - Lists all calendars with properties
- **DiscoverCalendars** - Combines discovery operations

#### Event Operations
- **QueryCalendar** - Flexible calendar queries with filters
- **GetRecentEvents** - Retrieves events within time range
- **GetEventsByTimeRange** - Gets events between specific dates
- **GetEventByUID** - Finds specific event by UID
- **CountEvents** - Counts events in calendar
- **GetAllEvents** - Retrieves all events (4-year window)
- **SearchEvents** - Text search across events
- **GetUpcomingEvents** - Gets future events

### Technical Implementation

#### Core Functionality
- Initial CalDAV client implementation for iCloud
- Proper CalDAV XML generation with namespace handling
- Multi-status (207) response parsing
- Basic authentication support
- Zero external dependencies
- Context.Context support for all public methods
- Request cancellation and timeout support

#### Error Handling & Logging
- Custom error types with categorization (CalDAVError, MultiStatusError)
- Error helper methods (IsTemporary, IsAuthError, IsNotFound)
- Logging interface for debugging and monitoring
- Standard logger implementation with log levels
- HTTP request/response debug logging capability

#### Performance & Optimization
- Optimized HTTP client with connection pooling
- HTTP/2 support for better performance
- Client options for configuration (WithLogger, WithDebugLogging, WithHTTPClient)
- Connection pooling configuration options
- Performance benchmarks for critical operations

### Testing & Quality
- Comprehensive unit tests (85.6% coverage)
- 108 test functions across 12 test files
- Edge case testing for robust error handling
- Race condition testing (all tests pass with -race flag)
- Integration tests for iCloud compatibility
- 19 performance benchmarks
- Memory allocation optimizations

### Documentation & Tools
- Comprehensive README with usage examples
- Godoc documentation for all public APIs
- Example applications demonstrating usage
- Command-line tools for testing
- GitHub Actions CI/CD workflows
- MIT License

### iCloud Compatibility
- Proper VCALENDAR wrapping for iCloud requirements
- Time range handling for open-ended queries
- URL handling for both relative and absolute paths
- Fixes XML generation issues found in other CalDAV libraries
- Successfully tested with real iCloud accounts

### Known Limitations
- Read-only operations (no create/update/delete)
- No support for tasks/reminders
- No support for calendar sharing operations
- No support for recurring event modifications