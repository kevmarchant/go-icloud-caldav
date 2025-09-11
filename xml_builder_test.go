package caldav

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPropfindXML(t *testing.T) {
	tests := []struct {
		name     string
		props    []string
		expected []string
	}{
		{
			name:  "current user principal",
			props: []string{"current-user-principal"},
			expected: []string{
				`<D:current-user-principal/>`,
				`xmlns:D="DAV:"`,
			},
		},
		{
			name:  "calendar home set",
			props: []string{"calendar-home-set"},
			expected: []string{
				`<C:calendar-home-set/>`,
				`xmlns:C="urn:ietf:params:xml:ns:caldav"`,
			},
		},
		{
			name:  "multiple properties",
			props: []string{"displayname", "resourcetype", "getctag"},
			expected: []string{
				`<D:displayname/>`,
				`<D:resourcetype/>`,
				`<CS:getctag/>`,
				`xmlns:CS="http://calendarserver.org/ns/"`,
			},
		},
		{
			name:  "apple namespace properties",
			props: []string{"calendar-color", "calendar-order"},
			expected: []string{
				`<A:calendar-color/>`,
				`<A:calendar-order/>`,
				`xmlns:A="http://apple.com/ns/ical/"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildPropfindXML(tt.props)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resultStr := string(result)

			for _, expected := range tt.expected {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("expected XML to contain %q, got:\n%s", expected, resultStr)
				}
			}

			if !strings.HasPrefix(resultStr, `<?xml version="1.0" encoding="utf-8"?>`) {
				t.Error("XML should start with proper declaration")
			}

			if !strings.Contains(resultStr, `<D:propfind`) {
				t.Error("XML should have propfind root element")
			}
		})
	}
}

func TestBuildCalendarQueryXML(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, loc)
	endTime := time.Date(2025, 1, 31, 23, 59, 59, 0, loc)

	tests := []struct {
		name        string
		query       CalendarQuery
		expected    []string
		notExpected []string
	}{
		{
			name: "basic query with properties",
			query: CalendarQuery{
				Properties: []string{"getetag", "calendar-data"},
			},
			expected: []string{
				`<D:getetag/>`,
				`<C:calendar-data/>`,
				`<C:calendar-query`,
			},
			notExpected: []string{
				`<C:filter>`,
			},
		},
		{
			name: "query with component filter",
			query: CalendarQuery{
				Properties: []string{"calendar-data"},
				Filter: Filter{
					Component: "VEVENT",
				},
			},
			expected: []string{
				`<C:filter>`,
				`<C:comp-filter name="VEVENT">`,
				`</C:comp-filter>`,
			},
		},
		{
			name: "query with prop-filter and text-match",
			query: CalendarQuery{
				Properties: []string{"calendar-data"},
				Filter: Filter{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name: "UID",
							TextMatch: &TextMatch{
								Value: "test-uid-123",
							},
						},
					},
				},
			},
			expected: []string{
				`<C:prop-filter name="UID">`,
				`<C:text-match>test-uid-123</C:text-match>`,
				`</C:prop-filter>`,
			},
		},
		{
			name: "query with time range",
			query: CalendarQuery{
				Properties: []string{"calendar-data"},
				TimeRange: &TimeRange{
					Start: startTime,
					End:   endTime,
				},
			},
			expected: []string{
				`<C:time-range start="20250101T000000Z" end="20250131T235959Z"/>`,
				`<C:comp-filter name="VCALENDAR">`,
				`<C:comp-filter name="VEVENT">`,
			},
		},
		{
			name: "query with negated text match",
			query: CalendarQuery{
				Properties: []string{"calendar-data"},
				Filter: Filter{
					Component: "VEVENT",
					Props: []PropFilter{
						{
							Name: "SUMMARY",
							TextMatch: &TextMatch{
								Value:           "meeting",
								NegateCondition: true,
								Collation:       "i;ascii-casemap",
							},
						},
					},
				},
			},
			expected: []string{
				`<C:prop-filter name="SUMMARY">`,
				`<C:text-match collation="i;ascii-casemap" negate-condition="yes">meeting</C:text-match>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildCalendarQueryXML(tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resultStr := string(result)

			for _, expected := range tt.expected {
				if !strings.Contains(resultStr, expected) {
					t.Errorf("expected XML to contain %q, got:\n%s", expected, resultStr)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(resultStr, notExpected) {
					t.Errorf("expected XML not to contain %q, got:\n%s", notExpected, resultStr)
				}
			}

			if !strings.HasPrefix(resultStr, `<?xml version="1.0" encoding="utf-8"?>`) {
				t.Error("XML should start with proper declaration")
			}

			if !strings.Contains(resultStr, `xmlns:C="urn:ietf:params:xml:ns:caldav"`) {
				t.Error("XML should declare CalDAV namespace")
			}
		})
	}
}

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple text", "simple text"},
		{"<tag>", "&lt;tag&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&#34;quoted&#34;"},
		{"'apostrophe'", "&#39;apostrophe&#39;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := xmlEscape(tt.input)
			if result != tt.expected {
				t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatTimeForCalDAV(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	testTime := time.Date(2025, 1, 15, 14, 30, 45, 0, loc)

	result := formatTimeForCalDAV(testTime)
	expected := "20250115T143045Z"

	if result != expected {
		t.Errorf("formatTimeForCalDAV() = %q, want %q", result, expected)
	}

	locNY, _ := time.LoadLocation("America/New_York")
	testTimeNY := time.Date(2025, 1, 15, 14, 30, 45, 0, locNY)
	resultNY := formatTimeForCalDAV(testTimeNY)
	expectedNY := "20250115T193045Z"

	if resultNY != expectedNY {
		t.Errorf("formatTimeForCalDAV() with NY time = %q, want %q", resultNY, expectedNY)
	}
}
