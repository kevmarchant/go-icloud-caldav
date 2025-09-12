package caldav

import (
	"strings"
	"testing"
)

func TestXMLValidator_ValidateCalDAVRequest(t *testing.T) {
	tests := []struct {
		name          string
		xmlData       string
		autoCorrect   bool
		strictMode    bool
		expectValid   bool
		expectErrors  int
		errorContains []string
	}{
		{
			name: "valid calendar query",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="20250101T000000Z" end="20250131T235959Z"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:  false,
			strictMode:   false,
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "double nested VCALENDAR without autocorrect",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VCALENDAR">
        <C:comp-filter name="VEVENT"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"Double-nested VCALENDAR"},
		},
		{
			name: "double nested VCALENDAR with autocorrect",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VCALENDAR">
        <C:comp-filter name="VEVENT"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:  true,
			strictMode:   false,
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "missing CalDAV namespace",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
</C:calendar-query>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"Missing required namespace"},
		},
		{
			name: "invalid time format without autocorrect",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="2025-01-01T00:00:00" end="2025-01-31T23:59:59"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  2,
			errorContains: []string{"Invalid time format"},
		},
		{
			name: "invalid time format with autocorrect",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <C:time-range start="2025-01-01T00:00:00" end="2025-01-31T23:59:59"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:  true,
			strictMode:   false,
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "incorrect prop filter syntax",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        <prop name="UID">
          <C:text-match>test-uid</C:text-match>
        </prop>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:  true,
			strictMode:   false,
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "valid propfind request",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:displayname/>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`,
			autoCorrect:  false,
			strictMode:   false,
			expectValid:  true,
			expectErrors: 0,
		},
		{
			name: "propfind missing DAV namespace",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<D:propfind>
  <D:prop>
    <D:displayname/>
  </D:prop>
</D:propfind>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"Missing DAV: namespace"},
		},
		{
			name: "propfind missing prop element",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
</D:propfind>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"PROPFIND must contain a prop element"},
		},
		{
			name: "malformed XML",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
</C:calendar-query>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"not well-formed"},
		},
		{
			name: "filter without VCALENDAR root",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VEVENT">
      <C:time-range start="20250101T000000Z" end="20250131T235959Z"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect:   false,
			strictMode:    false,
			expectValid:   false,
			expectErrors:  1,
			errorContains: []string{"must have VCALENDAR as root component"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewXMLValidator(tt.autoCorrect, tt.strictMode)
			result, _ := validator.ValidateCalDAVRequest([]byte(tt.xmlData))

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, result.Valid)
			}

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}

			for _, expectedError := range tt.errorContains {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(err.Message, expectedError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q not found in %v", expectedError, result.Errors)
				}
			}
		})
	}
}

func TestXMLValidator_AutoCorrection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "correct double nested VCALENDAR",
			input: `<C:comp-filter name="VCALENDAR">
<C:comp-filter name="VCALENDAR">
<C:comp-filter name="VEVENT"/>
</C:comp-filter>
</C:comp-filter>`,
			expected: `<C:comp-filter name="VCALENDAR">
<C:comp-filter name="VEVENT"/>
</C:comp-filter>`,
		},
		{
			name:     "correct incorrect prop filter",
			input:    `<prop name="UID">`,
			expected: `<C:prop-filter name="UID">`,
		},
		{
			name:     "correct prop closing tag",
			input:    `</prop>`,
			expected: `</C:prop-filter>`,
		},
		{
			name:     "correct time format with dashes",
			input:    `start="2025-01-01T00:00:00"`,
			expected: `start="20250101T000000Z"`,
		},
		{
			name:     "correct time format with colons",
			input:    `end="2025-01-31T23:59:59"`,
			expected: `end="20250131T235959Z"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewXMLValidator(true, false)
			corrected := validator.autoCorrectCommonIssues([]byte(tt.input))

			if !strings.Contains(string(corrected), tt.expected) {
				t.Errorf("expected corrected XML to contain %q, got %q", tt.expected, string(corrected))
			}
		})
	}
}

func TestXMLValidator_StrictMode(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:"  xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
</C:calendar-query>`

	validator := NewXMLValidator(false, true)
	result, _ := validator.ValidateCalDAVRequest([]byte(xmlData))

	if len(result.Warnings) == 0 {
		t.Error("expected warnings in strict mode for unnecessary whitespace")
	}

	foundWarning := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "unnecessary whitespace") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Errorf("expected warning about whitespace, got: %v", result.Warnings)
	}
}

func TestValidateAndCorrectXML(t *testing.T) {
	tests := []struct {
		name        string
		xmlData     string
		autoCorrect bool
		expectError bool
	}{
		{
			name: "valid XML without correction",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
</C:calendar-query>`,
			autoCorrect: false,
			expectError: false,
		},
		{
			name: "invalid XML without correction",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VCALENDAR">
        <C:comp-filter name="VEVENT"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect: false,
			expectError: true,
		},
		{
			name: "invalid XML with correction",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VCALENDAR">
        <C:comp-filter name="VEVENT"/>
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`,
			autoCorrect: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndCorrectXML([]byte(tt.xmlData), tt.autoCorrect)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestXMLValidator_TimeRangeValidation(t *testing.T) {
	tests := []struct {
		name        string
		timeRange   string
		expectValid bool
	}{
		{
			name:        "valid CalDAV time format",
			timeRange:   `<C:time-range start="20250101T000000Z" end="20250131T235959Z"/>`,
			expectValid: true,
		},
		{
			name:        "invalid format with dashes",
			timeRange:   `<C:time-range start="2025-01-01T00:00:00Z" end="2025-01-31T23:59:59Z"/>`,
			expectValid: false,
		},
		{
			name:        "invalid format with colons",
			timeRange:   `<C:time-range start="20250101T00:00:00Z" end="20250131T23:59:59Z"/>`,
			expectValid: false,
		},
		{
			name:        "missing Z suffix",
			timeRange:   `<C:time-range start="20250101T000000" end="20250131T235959"/>`,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xmlData := `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT">
        ` + tt.timeRange + `
      </C:comp-filter>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`

			validator := NewXMLValidator(false, false)
			result, _ := validator.ValidateCalDAVRequest([]byte(xmlData))

			if tt.expectValid && !result.Valid {
				t.Errorf("expected valid time range, got errors: %v", result.Errors)
			}

			if !tt.expectValid && result.Valid {
				t.Error("expected invalid time range, but validation passed")
			}
		})
	}
}
