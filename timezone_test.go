package caldav

import (
	"testing"
	"time"
)

func TestParseOffset(t *testing.T) {
	tests := []struct {
		name     string
		offset   string
		expected time.Duration
	}{
		{
			name:     "positive hours and minutes",
			offset:   "+0530",
			expected: 5*time.Hour + 30*time.Minute,
		},
		{
			name:     "negative hours and minutes",
			offset:   "-0800",
			expected: -8 * time.Hour,
		},
		{
			name:     "with seconds",
			offset:   "+051530",
			expected: 5*time.Hour + 15*time.Minute + 30*time.Second,
		},
		{
			name:     "UTC",
			offset:   "+0000",
			expected: 0,
		},
		{
			name:     "invalid format",
			offset:   "invalid",
			expected: 0,
		},
		{
			name:     "empty",
			offset:   "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOffset(tt.offset)
			if result != tt.expected {
				t.Errorf("parseOffset(%q) = %v, expected %v", tt.offset, result, tt.expected)
			}
		})
	}
}

func TestCreateTimeZoneInfo(t *testing.T) {
	// Test timezone with STANDARD and DAYLIGHT components
	standardStart := time.Date(2023, 11, 5, 2, 0, 0, 0, time.UTC)
	daylightStart := time.Date(2023, 3, 12, 2, 0, 0, 0, time.UTC)

	tz := ParsedTimeZone{
		TZID: "America/New_York",
		StandardTime: ParsedTimeZoneComponent{
			DTStart:          &standardStart,
			TZOffsetFrom:     "-0400",
			TZOffsetTo:       "-0500",
			TZName:           "EST",
			RecurrenceRule:   "FREQ=YEARLY;BYMONTH=11;BYDAY=1SU",
			CustomProperties: make(map[string]string),
		},
		DaylightTime: ParsedTimeZoneComponent{
			DTStart:          &daylightStart,
			TZOffsetFrom:     "-0500",
			TZOffsetTo:       "-0400",
			TZName:           "EDT",
			RecurrenceRule:   "FREQ=YEARLY;BYMONTH=3;BYDAY=2SU",
			CustomProperties: make(map[string]string),
		},
		CustomProperties: make(map[string]string),
	}

	tzInfo := CreateTimeZoneInfo(tz)

	if tzInfo.TZID != "America/New_York" {
		t.Errorf("TZID: expected 'America/New_York', got %s", tzInfo.TZID)
	}

	if len(tzInfo.Transitions) == 0 {
		t.Error("Expected transitions to be created")
	}

	// Check that we have both standard and daylight transitions
	hasStandard := false
	hasDaylight := false
	for _, transition := range tzInfo.Transitions {
		if !transition.IsDST && transition.Abbreviation == "EST" {
			hasStandard = true
		}
		if transition.IsDST && transition.Abbreviation == "EDT" {
			hasDaylight = true
		}
	}

	if !hasStandard {
		t.Error("Expected to find standard time transitions")
	}
	if !hasDaylight {
		t.Error("Expected to find daylight time transitions")
	}
}

func TestTimeZoneInfo_GetOffsetAtTime(t *testing.T) {
	// Create a simple timezone info with known transitions
	tzInfo := &TimeZoneInfo{
		TZID: "Test/Zone",
		Transitions: []TimeZoneTransition{
			{
				DateTime:   time.Date(2023, 3, 12, 2, 0, 0, 0, time.UTC),
				OffsetFrom: -5 * time.Hour,
				OffsetTo:   -4 * time.Hour,
				IsDST:      true,
			},
			{
				DateTime:   time.Date(2023, 11, 5, 2, 0, 0, 0, time.UTC),
				OffsetFrom: -4 * time.Hour,
				OffsetTo:   -5 * time.Hour,
				IsDST:      false,
			},
		},
	}

	tests := []struct {
		name     string
		testTime time.Time
		expected time.Duration
	}{
		{
			name:     "before any transitions",
			testTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: -5 * time.Hour, // OffsetFrom of first transition
		},
		{
			name:     "during DST",
			testTime: time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			expected: -4 * time.Hour,
		},
		{
			name:     "during standard time",
			testTime: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			expected: -5 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tzInfo.GetOffsetAtTime(tt.testTime)
			if result != tt.expected {
				t.Errorf("GetOffsetAtTime(%v) = %v, expected %v", tt.testTime, result, tt.expected)
			}
		})
	}
}

func TestTimeZoneInfo_ConvertToUTC(t *testing.T) {
	// Create timezone info for EST/EDT (UTC-5/UTC-4)
	tzInfo := &TimeZoneInfo{
		TZID: "America/New_York",
		Transitions: []TimeZoneTransition{
			{
				DateTime:   time.Date(2023, 3, 12, 2, 0, 0, 0, time.UTC),
				OffsetFrom: -5 * time.Hour,
				OffsetTo:   -4 * time.Hour,
				IsDST:      true,
			},
		},
	}

	// Test conversion during DST
	localTime := time.Date(2023, 7, 1, 12, 0, 0, 0, time.UTC) // Simulated local time
	expectedUTC := localTime.Add(4 * time.Hour)               // Add 4 hours to get UTC

	resultUTC := tzInfo.ConvertToUTC(localTime)
	if !resultUTC.Equal(expectedUTC) {
		t.Errorf("ConvertToUTC: expected %v, got %v", expectedUTC, resultUTC)
	}
}

func TestTimeZoneInfo_IsDSTAtTime(t *testing.T) {
	tzInfo := &TimeZoneInfo{
		TZID: "America/New_York",
		Transitions: []TimeZoneTransition{
			{
				DateTime:   time.Date(2023, 3, 12, 2, 0, 0, 0, time.UTC),
				OffsetFrom: -5 * time.Hour,
				OffsetTo:   -4 * time.Hour,
				IsDST:      true,
			},
			{
				DateTime:   time.Date(2023, 11, 5, 2, 0, 0, 0, time.UTC),
				OffsetFrom: -4 * time.Hour,
				OffsetTo:   -5 * time.Hour,
				IsDST:      false,
			},
		},
	}

	tests := []struct {
		name     string
		testTime time.Time
		expected bool
	}{
		{
			name:     "before DST starts",
			testTime: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "during DST",
			testTime: time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "after DST ends",
			testTime: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tzInfo.IsDSTAtTime(tt.testTime)
			if result != tt.expected {
				t.Errorf("IsDSTAtTime(%v) = %v, expected %v", tt.testTime, result, tt.expected)
			}
		})
	}
}

func TestLoadLocationFromTZID(t *testing.T) {
	tests := []struct {
		name      string
		tzid      string
		expectErr bool
	}{
		{
			name:      "valid IANA timezone",
			tzid:      "America/New_York",
			expectErr: false,
		},
		{
			name:      "US timezone alias",
			tzid:      "US/Eastern",
			expectErr: false,
		},
		{
			name:      "invalid timezone",
			tzid:      "Invalid/Timezone",
			expectErr: true,
		},
		{
			name:      "empty timezone",
			tzid:      "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := LoadLocationFromTZID(tt.tzid)
			if tt.expectErr {
				if err == nil {
					t.Errorf("LoadLocationFromTZID(%q) expected error but got none", tt.tzid)
				}
			} else {
				if err != nil {
					t.Errorf("LoadLocationFromTZID(%q) = error %v", tt.tzid, err)
				}
				if loc == nil {
					t.Errorf("LoadLocationFromTZID(%q) = nil location", tt.tzid)
				}
			}
		})
	}
}

func TestParsedTimeZone_EnhancedStructure(t *testing.T) {
	// Test that the enhanced ParsedTimeZoneComponent structure is working
	component := ParsedTimeZoneComponent{
		RecurrenceDates:  []time.Time{time.Now()},
		ExceptionDates:   []time.Time{time.Now()},
		Comment:          []string{"Test comment"},
		CustomProperties: make(map[string]string),
	}

	if len(component.RecurrenceDates) != 1 {
		t.Error("RecurrenceDates not properly initialized")
	}
	if len(component.ExceptionDates) != 1 {
		t.Error("ExceptionDates not properly initialized")
	}
	if len(component.Comment) != 1 {
		t.Error("Comment not properly initialized")
	}
	if component.CustomProperties == nil {
		t.Error("CustomProperties not properly initialized")
	}
}
