package caldav

import (
	"reflect"
	"testing"
	"time"
)

func TestParseRRule(t *testing.T) {
	tests := []struct {
		name     string
		rruleStr string
		want     RRule
	}{
		{
			name:     "Weekly recurrence",
			rruleStr: "FREQ=WEEKLY",
			want: RRule{
				Freq:     "WEEKLY",
				Interval: 1,
			},
		},
		{
			name:     "Daily with count",
			rruleStr: "FREQ=DAILY;COUNT=5",
			want: RRule{
				Freq:     "DAILY",
				Interval: 1,
				Count:    5,
			},
		},
		{
			name:     "Weekly with until date",
			rruleStr: "FREQ=WEEKLY;UNTIL=20250725T084500Z",
			want: RRule{
				Freq:     "WEEKLY",
				Interval: 1,
			},
		},
		{
			name:     "Monthly with interval",
			rruleStr: "FREQ=MONTHLY;INTERVAL=2",
			want: RRule{
				Freq:     "MONTHLY",
				Interval: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRRule(tt.rruleStr)
			if err != nil {
				t.Errorf("ParseRRule() error = %v", err)
				return
			}
			if got.Freq != tt.want.Freq {
				t.Errorf("ParseRRule() Freq = %v, want %v", got.Freq, tt.want.Freq)
			}
			if got.Interval != tt.want.Interval {
				t.Errorf("ParseRRule() Interval = %v, want %v", got.Interval, tt.want.Interval)
			}
			if got.Count != tt.want.Count {
				t.Errorf("ParseRRule() Count = %v, want %v", got.Count, tt.want.Count)
			}
		})
	}
}

func TestExpandEvent(t *testing.T) {
	baseTime := time.Date(2023, 12, 14, 17, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 12, 14, 17, 30, 0, 0, time.UTC)

	tests := []struct {
		name           string
		event          ParsedEvent
		start          time.Time
		end            time.Time
		wantCount      int
		checkFirstDate bool
		firstDate      time.Time
	}{
		{
			name: "Weekly team meeting",
			event: ParsedEvent{
				Summary:        "Team Meeting",
				DTStart:        &baseTime,
				DTEnd:          &endTime,
				RecurrenceRule: "FREQ=WEEKLY",
			},
			start:          time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC),
			end:            time.Date(2025, 9, 30, 0, 0, 0, 0, time.UTC),
			wantCount:      4,
			checkFirstDate: true,
			firstDate:      time.Date(2025, 9, 4, 17, 0, 0, 0, time.UTC),
		},
		{
			name: "Daily with count",
			event: ParsedEvent{
				Summary:        "Daily task",
				DTStart:        &baseTime,
				RecurrenceRule: "FREQ=DAILY;COUNT=5",
			},
			start:     time.Date(2023, 12, 14, 0, 0, 0, 0, time.UTC),
			end:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 5,
		},
		{
			name: "Non-recurring event",
			event: ParsedEvent{
				Summary: "Single event",
				DTStart: &baseTime,
			},
			start:     time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			end:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandEvent(tt.event, tt.start, tt.end)
			if err != nil {
				t.Errorf("ExpandEvent() error = %v", err)
				return
			}
			if len(got) != tt.wantCount {
				t.Errorf("ExpandEvent() returned %d events, want %d", len(got), tt.wantCount)
			}

			if tt.checkFirstDate && len(got) > 0 && got[0].DTStart != nil {
				if !got[0].DTStart.Equal(tt.firstDate) {
					t.Errorf("First occurrence date = %v, want %v",
						got[0].DTStart.Format(time.RFC3339),
						tt.firstDate.Format(time.RFC3339))
				}
			}

			for i, event := range got {
				if event.RecurrenceID == nil && tt.event.RecurrenceRule != "" {
					t.Errorf("Event %d missing RecurrenceID", i)
				}
				if event.Summary != tt.event.Summary {
					t.Errorf("Event %d summary = %v, want %v", i, event.Summary, tt.event.Summary)
				}
			}
		})
	}
}

func TestExpandEventForSeptember2025(t *testing.T) {
	meetingTime := time.Date(2023, 12, 14, 17, 0, 0, 0, time.UTC)
	meetingEndTime := time.Date(2023, 12, 14, 17, 30, 0, 0, time.UTC)

	event := ParsedEvent{
		Summary:        "Weekly Team Standup",
		DTStart:        &meetingTime,
		DTEnd:          &meetingEndTime,
		RecurrenceRule: "FREQ=WEEKLY",
	}

	start := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 9, 30, 0, 0, 0, 0, time.UTC)

	occurrences, err := ExpandEvent(event, start, end)
	if err != nil {
		t.Fatalf("ExpandEvent() error = %v", err)
	}

	found := false
	expectedDate := time.Date(2025, 9, 11, 17, 0, 0, 0, time.UTC)

	for _, occ := range occurrences {
		if occ.DTStart.Equal(expectedDate) {
			found = true
			if occ.Summary != "Weekly Team Standup" {
				t.Errorf("Event summary = %v, want 'Weekly Team Standup'", occ.Summary)
			}
			break
		}
	}

	if !found {
		t.Errorf("No occurrence found on Sept 11, 2025. Got %d occurrences:", len(occurrences))
		for i, occ := range occurrences {
			t.Errorf("  [%d] %v", i, occ.DTStart.Format(time.RFC3339))
		}
	}
}

func TestExpandEventWithRDates(t *testing.T) {
	baseTime := time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		event     ParsedEvent
		start     time.Time
		end       time.Time
		wantDates []time.Time
	}{
		{
			name: "Event with only RDATEs",
			event: ParsedEvent{
				Summary: "Special Meeting",
				DTStart: &baseTime,
				RecurrenceDates: []time.Time{
					time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
					time.Date(2025, 1, 22, 14, 0, 0, 0, time.UTC),
					time.Date(2025, 2, 5, 14, 0, 0, 0, time.UTC),
				},
			},
			start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			wantDates: []time.Time{
				time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 22, 14, 0, 0, 0, time.UTC),
				time.Date(2025, 2, 5, 14, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "RRULE with additional RDATEs",
			event: ParsedEvent{
				Summary:        "Weekly + Special",
				DTStart:        &baseTime,
				RecurrenceRule: "FREQ=WEEKLY",
				RecurrenceDates: []time.Time{
					time.Date(2025, 1, 14, 14, 0, 0, 0, time.UTC), // Tuesday (extra)
					time.Date(2025, 1, 28, 14, 0, 0, 0, time.UTC), // Tuesday (extra)
				},
			},
			start: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC),
			wantDates: []time.Time{
				time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC), // Friday (base)
				time.Date(2025, 1, 14, 14, 0, 0, 0, time.UTC), // Tuesday (RDATE)
				time.Date(2025, 1, 17, 14, 0, 0, 0, time.UTC), // Friday (RRULE)
				time.Date(2025, 1, 24, 14, 0, 0, 0, time.UTC), // Friday (RRULE)
				time.Date(2025, 1, 28, 14, 0, 0, 0, time.UTC), // Tuesday (RDATE)
				time.Date(2025, 1, 31, 14, 0, 0, 0, time.UTC), // Friday (RRULE)
			},
		},
		{
			name: "RDATEs with EXDATE exclusions",
			event: ParsedEvent{
				Summary: "Special Dates with Exclusions",
				DTStart: &baseTime,
				RecurrenceDates: []time.Time{
					time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
					time.Date(2025, 1, 22, 14, 0, 0, 0, time.UTC),
					time.Date(2025, 1, 29, 14, 0, 0, 0, time.UTC),
				},
				ExceptionDates: []time.Time{
					time.Date(2025, 1, 22, 14, 0, 0, 0, time.UTC), // Exclude middle date
				},
			},
			start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			wantDates: []time.Time{
				time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 29, 14, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandEvent(tt.event, tt.start, tt.end)
			if err != nil {
				t.Errorf("ExpandEvent() error = %v", err)
				return
			}

			if len(got) != len(tt.wantDates) {
				t.Errorf("ExpandEvent() returned %d events, want %d", len(got), len(tt.wantDates))
				for i, event := range got {
					if event.DTStart != nil {
						t.Errorf("  Got[%d]: %v", i, event.DTStart.Format(time.RFC3339))
					}
				}
				return
			}

			// Check that all expected dates are present
			for _, wantDate := range tt.wantDates {
				found := false
				for _, event := range got {
					if event.DTStart != nil && event.DTStart.Equal(wantDate) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected date %v not found in results", wantDate.Format(time.RFC3339))
				}
			}
		})
	}
}

func TestExpandEventWithEXRULE(t *testing.T) {
	baseTime := time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC) // Monday

	tests := []struct {
		name         string
		event        ParsedEvent
		start        time.Time
		end          time.Time
		wantCount    int
		checkDates   []time.Time
		excludeDates []time.Time
	}{
		{
			name: "Daily event with weekly EXRULE",
			event: ParsedEvent{
				Summary:        "Daily except Mondays",
				DTStart:        &baseTime,
				RecurrenceRule: "FREQ=DAILY",
				ExceptionRule:  "FREQ=WEEKLY;BYDAY=MO",
			},
			start:     time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
			end:       time.Date(2025, 1, 19, 23, 59, 59, 0, time.UTC),
			wantCount: 12, // 14 days minus 2 Mondays (6th and 13th)
			checkDates: []time.Time{
				time.Date(2025, 1, 7, 10, 0, 0, 0, time.UTC),  // Tuesday
				time.Date(2025, 1, 8, 10, 0, 0, 0, time.UTC),  // Wednesday
				time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC), // Tuesday
			},
			excludeDates: []time.Time{
				time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC),  // Monday (excluded)
				time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC), // Monday (excluded)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandEvent(tt.event, tt.start, tt.end)
			if err != nil {
				t.Errorf("ExpandEvent() error = %v", err)
				return
			}

			if len(got) != tt.wantCount {
				t.Errorf("ExpandEvent() returned %d events, want %d", len(got), tt.wantCount)
				for i, event := range got {
					if event.DTStart != nil {
						t.Logf("  Got[%d]: %v (%s)", i, event.DTStart.Format(time.RFC3339), event.DTStart.Weekday())
					}
				}
			}

			// Check that expected dates are present
			for _, wantDate := range tt.checkDates {
				found := false
				for _, event := range got {
					if event.DTStart != nil && event.DTStart.Equal(wantDate) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected date %v not found in results", wantDate.Format(time.RFC3339))
				}
			}

			// Check that excluded dates are NOT present
			for _, excludeDate := range tt.excludeDates {
				for _, event := range got {
					if event.DTStart != nil && event.DTStart.Equal(excludeDate) {
						t.Errorf("Excluded date %v should not be in results", excludeDate.Format(time.RFC3339))
					}
				}
			}
		})
	}
}

func TestExpandEventWithModifiedInstance(t *testing.T) {
	meetingTime := time.Date(2023, 12, 14, 17, 0, 0, 0, time.UTC)
	meetingEndTime := time.Date(2023, 12, 14, 17, 30, 0, 0, time.UTC)

	// Master recurring event
	masterEvent := ParsedEvent{
		UID:            "team-meeting-weekly",
		Summary:        "Weekly Team Meeting",
		DTStart:        &meetingTime,
		DTEnd:          &meetingEndTime,
		RecurrenceRule: "FREQ=WEEKLY",
	}

	// Modified instance for Sept 11, 2025
	sept11Time := time.Date(2025, 9, 11, 17, 0, 0, 0, time.UTC)
	sept11EndTime := time.Date(2025, 9, 11, 17, 30, 0, 0, time.UTC)
	modifiedEvent := ParsedEvent{
		UID:          "team-meeting-weekly",
		Summary:      "Weekly Team Meeting (Room 201 - Main conference room unavailable)",
		DTStart:      &sept11Time,
		DTEnd:        &sept11EndTime,
		RecurrenceID: &sept11Time,
	}

	// Create exceptions map
	exceptions := map[string]*ParsedEvent{
		"20250911T170000Z": &modifiedEvent,
	}

	start := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 9, 30, 0, 0, 0, 0, time.UTC)

	occurrences, err := ExpandEventWithExceptions(masterEvent, exceptions, start, end)
	if err != nil {
		t.Fatalf("ExpandEventWithExceptions() error = %v", err)
	}

	found := false
	expectedDate := time.Date(2025, 9, 11, 17, 0, 0, 0, time.UTC)

	for _, occ := range occurrences {
		if occ.DTStart.Equal(expectedDate) {
			found = true
			if occ.Summary != "Weekly Team Meeting (Room 201 - Main conference room unavailable)" {
				t.Errorf("Event summary = %v, want 'Weekly Team Meeting (Room 201 - Main conference room unavailable)'", occ.Summary)
			}
			break
		}
	}

	if !found {
		t.Errorf("No occurrence found on Sept 11, 2025. Got %d occurrences:", len(occurrences))
		for i, occ := range occurrences {
			t.Errorf("  [%d] %v: %s", i, occ.DTStart.Format(time.RFC3339), occ.Summary)
		}
	}

	// Also verify other occurrences have the original summary
	for _, occ := range occurrences {
		if !occ.DTStart.Equal(expectedDate) {
			if occ.Summary != "Weekly Team Meeting" {
				t.Errorf("Non-modified occurrence has wrong summary: %v", occ.Summary)
			}
		}
	}
}

func TestAdvancedRRuleFeatures(t *testing.T) {
	tests := []struct {
		name     string
		rruleStr string
		expected RRule
	}{
		{
			name:     "Parse BYHOUR",
			rruleStr: "FREQ=DAILY;BYHOUR=9,15",
			expected: RRule{
				Freq:     "DAILY",
				Interval: 1,
				ByHour:   []int{9, 15},
			},
		},
		{
			name:     "Parse BYMINUTE",
			rruleStr: "FREQ=HOURLY;BYMINUTE=0,30",
			expected: RRule{
				Freq:     "HOURLY",
				Interval: 1,
				ByMinute: []int{0, 30},
			},
		},
		{
			name:     "Parse BYSECOND",
			rruleStr: "FREQ=MINUTELY;BYSECOND=0,15,30,45",
			expected: RRule{
				Freq:     "MINUTELY",
				Interval: 1,
				BySecond: []int{0, 15, 30, 45},
			},
		},
		{
			name:     "Parse WKST",
			rruleStr: "FREQ=WEEKLY;WKST=SU;BYDAY=MO,WE,FR",
			expected: RRule{
				Freq:      "WEEKLY",
				Interval:  1,
				ByDay:     []string{"MO", "WE", "FR"},
				WeekStart: "SU",
			},
		},
		{
			name:     "Parse BYSETPOS",
			rruleStr: "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1",
			expected: RRule{
				Freq:     "MONTHLY",
				Interval: 1,
				ByDay:    []string{"MO", "TU", "WE", "TH", "FR"},
				BySetPos: []int{-1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRRule(tt.rruleStr)
			if err != nil {
				t.Fatalf("ParseRRule() error = %v", err)
			}

			if got.Freq != tt.expected.Freq {
				t.Errorf("Freq = %v, want %v", got.Freq, tt.expected.Freq)
			}

			if !reflect.DeepEqual(got.ByHour, tt.expected.ByHour) {
				t.Errorf("ByHour = %v, want %v", got.ByHour, tt.expected.ByHour)
			}

			if !reflect.DeepEqual(got.ByMinute, tt.expected.ByMinute) {
				t.Errorf("ByMinute = %v, want %v", got.ByMinute, tt.expected.ByMinute)
			}

			if !reflect.DeepEqual(got.BySecond, tt.expected.BySecond) {
				t.Errorf("BySecond = %v, want %v", got.BySecond, tt.expected.BySecond)
			}

			if got.WeekStart != tt.expected.WeekStart {
				t.Errorf("WeekStart = %v, want %v", got.WeekStart, tt.expected.WeekStart)
			}

			if !reflect.DeepEqual(got.BySetPos, tt.expected.BySetPos) {
				t.Errorf("BySetPos = %v, want %v", got.BySetPos, tt.expected.BySetPos)
			}
		})
	}
}

func TestComplexByDayPatterns(t *testing.T) {
	tests := []struct {
		name          string
		rruleStr      string
		startTime     time.Time
		expectedDates []time.Time
		count         int
	}{
		{
			name:      "Second Tuesday of month",
			rruleStr:  "FREQ=MONTHLY;BYDAY=2TU",
			startTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC), // 2nd Tuesday of January
				time.Date(2025, 2, 11, 10, 0, 0, 0, time.UTC), // 2nd Tuesday of February
				time.Date(2025, 3, 11, 10, 0, 0, 0, time.UTC), // 2nd Tuesday of March
			},
			count: 3,
		},
		{
			name:      "Last Friday of month",
			rruleStr:  "FREQ=MONTHLY;BYDAY=-1FR",
			startTime: time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2025, 1, 31, 15, 0, 0, 0, time.UTC), // Last Friday of January
				time.Date(2025, 2, 28, 15, 0, 0, 0, time.UTC), // Last Friday of February
				time.Date(2025, 3, 28, 15, 0, 0, 0, time.UTC), // Last Friday of March
			},
			count: 3,
		},
		{
			name:      "Last weekday of month with BYSETPOS",
			rruleStr:  "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1",
			startTime: time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2025, 1, 31, 9, 0, 0, 0, time.UTC), // Last weekday of January (Friday)
				time.Date(2025, 2, 28, 9, 0, 0, 0, time.UTC), // Last weekday of February (Friday)
				time.Date(2025, 3, 31, 9, 0, 0, 0, time.UTC), // Last weekday of March (Monday)
			},
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ParsedEvent{
				UID:            "complex-pattern-test",
				Summary:        "Complex Pattern Event",
				DTStart:        &tt.startTime,
				RecurrenceRule: tt.rruleStr,
			}

			endTime := tt.startTime.AddDate(1, 0, 0) // 1 year range
			occurrences, err := ExpandEvent(event, tt.startTime, endTime)
			if err != nil {
				t.Fatalf("ExpandEvent() error = %v", err)
			}

			// Check we got at least the expected count
			if len(occurrences) < tt.count {
				t.Errorf("Got %d occurrences, expected at least %d", len(occurrences), tt.count)
			}

			// Check the first few expected dates
			for i, expectedDate := range tt.expectedDates {
				if i >= len(occurrences) {
					t.Errorf("Missing occurrence %d: expected %v", i, expectedDate)
					continue
				}

				if !occurrences[i].DTStart.Equal(expectedDate) {
					t.Errorf("Occurrence %d: got %v, want %v", i, occurrences[i].DTStart, expectedDate)
				}
			}
		})
	}
}

func TestTimeGranularityExpansion(t *testing.T) {
	tests := []struct {
		name          string
		rruleStr      string
		startTime     time.Time
		expectedCount int
		checkTimes    []time.Time
	}{
		{
			name:          "Daily at 9am and 3pm",
			rruleStr:      "FREQ=DAILY;BYHOUR=9,15;COUNT=4",
			startTime:     time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC),
			expectedCount: 4,
			checkTimes: []time.Time{
				time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 2, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 2, 15, 0, 0, 0, time.UTC),
			},
		},
		{
			name:          "Hourly at 0 and 30 minutes",
			rruleStr:      "FREQ=DAILY;BYHOUR=9,10;BYMINUTE=0,30;COUNT=4",
			startTime:     time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC),
			expectedCount: 4, // 2 hours x 2 minutes each = 4 occurrences on first day
			checkTimes: []time.Time{
				time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 1, 9, 30, 0, 0, time.UTC),
				time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ParsedEvent{
				UID:            "time-granularity-test",
				Summary:        "Time Granularity Event",
				DTStart:        &tt.startTime,
				RecurrenceRule: tt.rruleStr,
			}

			endTime := tt.startTime.AddDate(0, 0, 7) // 1 week range
			occurrences, err := ExpandEvent(event, tt.startTime, endTime)
			if err != nil {
				t.Fatalf("ExpandEvent() error = %v", err)
			}

			// Check count if specified
			if tt.expectedCount > 0 && len(occurrences) != tt.expectedCount {
				t.Errorf("Got %d occurrences, expected %d", len(occurrences), tt.expectedCount)
				for i, occ := range occurrences {
					t.Logf("  [%d] %v", i, occ.DTStart.Format(time.RFC3339))
				}
			}

			// Check specific times
			for _, checkTime := range tt.checkTimes {
				found := false
				for _, occ := range occurrences {
					if occ.DTStart.Equal(checkTime) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected occurrence at %v not found", checkTime)
				}
			}
		})
	}
}

func TestParseByDay(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []ByDayPart
	}{
		{
			name:  "Simple weekdays",
			input: []string{"MO", "WE", "FR"},
			expected: []ByDayPart{
				{Position: 0, Weekday: "MO"},
				{Position: 0, Weekday: "WE"},
				{Position: 0, Weekday: "FR"},
			},
		},
		{
			name:  "With positive positions",
			input: []string{"1MO", "2TU", "3WE"},
			expected: []ByDayPart{
				{Position: 1, Weekday: "MO"},
				{Position: 2, Weekday: "TU"},
				{Position: 3, Weekday: "WE"},
			},
		},
		{
			name:  "With negative positions",
			input: []string{"-1FR", "-2TH"},
			expected: []ByDayPart{
				{Position: -1, Weekday: "FR"},
				{Position: -2, Weekday: "TH"},
			},
		},
		{
			name:  "Mixed positions",
			input: []string{"MO", "2TU", "-1FR"},
			expected: []ByDayPart{
				{Position: 0, Weekday: "MO"},
				{Position: 2, Weekday: "TU"},
				{Position: -1, Weekday: "FR"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseByDay(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseByDay() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestGetNthWeekdayInMonth(t *testing.T) {
	tests := []struct {
		name     string
		year     int
		month    time.Month
		weekday  time.Weekday
		n        int
		expected time.Time
		found    bool
	}{
		{
			name:     "First Monday of January 2025",
			year:     2025,
			month:    time.January,
			weekday:  time.Monday,
			n:        1,
			expected: time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC),
			found:    true,
		},
		{
			name:     "Second Tuesday of January 2025",
			year:     2025,
			month:    time.January,
			weekday:  time.Tuesday,
			n:        2,
			expected: time.Date(2025, 1, 14, 0, 0, 0, 0, time.UTC),
			found:    true,
		},
		{
			name:     "Last Friday of January 2025",
			year:     2025,
			month:    time.January,
			weekday:  time.Friday,
			n:        -1,
			expected: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
			found:    true,
		},
		{
			name:     "Last Friday of February 2025",
			year:     2025,
			month:    time.February,
			weekday:  time.Friday,
			n:        -1,
			expected: time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
			found:    true,
		},
		{
			name:    "Fifth Monday of February 2025 (doesn't exist)",
			year:    2025,
			month:   time.February,
			weekday: time.Monday,
			n:       5,
			found:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := getNthWeekdayInMonth(tt.year, tt.month, tt.weekday, tt.n)
			if found != tt.found {
				t.Errorf("getNthWeekdayInMonth() found = %v, want %v", found, tt.found)
			}
			if found && !got.Equal(tt.expected) {
				t.Errorf("getNthWeekdayInMonth() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExpandEventWithInterval(t *testing.T) {
	tests := []struct {
		name          string
		event         ParsedEvent
		start         time.Time
		end           time.Time
		expectedDates []time.Time
	}{
		{
			name: "Biweekly Saturday (INTERVAL=2 with BYDAY)",
			event: ParsedEvent{
				Summary:        "Biweekly Event",
				DTStart:        timePtr(time.Date(2024, 9, 21, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=SA",
			},
			start: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2024, 9, 21, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 10, 5, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 10, 19, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 11, 2, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 11, 16, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 11, 30, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Every 3 days (DAILY with INTERVAL=3)",
			event: ParsedEvent{
				Summary:        "Every 3 Days",
				DTStart:        timePtr(time.Date(2024, 9, 1, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=DAILY;INTERVAL=3",
			},
			start: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2024, 9, 15, 0, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2024, 9, 1, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 4, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 7, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 10, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 13, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Bimonthly (MONTHLY with INTERVAL=2)",
			event: ParsedEvent{
				Summary:        "Bimonthly Event",
				DTStart:        timePtr(time.Date(2024, 9, 15, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=MONTHLY;INTERVAL=2",
			},
			start: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2024, 9, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 11, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Biennial (YEARLY with INTERVAL=2)",
			event: ParsedEvent{
				Summary:        "Biennial Event",
				DTStart:        timePtr(time.Date(2024, 9, 15, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=YEARLY;INTERVAL=2",
			},
			start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2024, 9, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC),
				time.Date(2028, 9, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Biweekly Tuesday and Thursday (INTERVAL=2 with multiple BYDAY)",
			event: ParsedEvent{
				Summary:        "Biweekly Multi-day",
				DTStart:        timePtr(time.Date(2024, 9, 3, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=TU,TH",
			},
			start: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2024, 10, 16, 0, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2024, 9, 3, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 5, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 17, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 19, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 10, 1, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 10, 3, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 10, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Weekly with INTERVAL=3 and BYDAY",
			event: ParsedEvent{
				Summary:        "Every 3 weeks on Monday",
				DTStart:        timePtr(time.Date(2024, 9, 2, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=WEEKLY;INTERVAL=3;BYDAY=MO",
			},
			start: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:   time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
			expectedDates: []time.Time{
				time.Date(2024, 9, 2, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 9, 23, 10, 0, 0, 0, time.UTC),
				time.Date(2024, 10, 14, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			occurrences, err := ExpandEvent(tt.event, tt.start, tt.end)
			if err != nil {
				t.Fatalf("ExpandEvent() error = %v", err)
			}

			if len(occurrences) != len(tt.expectedDates) {
				t.Errorf("ExpandEvent() returned %d events, want %d", len(occurrences), len(tt.expectedDates))
				for i, occ := range occurrences {
					if occ.DTStart != nil {
						t.Logf("  Got[%d]: %v (%s)", i, occ.DTStart.Format("2006-01-02"), occ.DTStart.Weekday())
					}
				}
				t.Logf("Expected dates:")
				for i, date := range tt.expectedDates {
					t.Logf("  Expected[%d]: %v (%s)", i, date.Format("2006-01-02"), date.Weekday())
				}
				return
			}

			for i, expectedDate := range tt.expectedDates {
				if i >= len(occurrences) {
					t.Errorf("Missing occurrence %d: expected %v", i, expectedDate.Format("2006-01-02"))
					continue
				}
				if occurrences[i].DTStart == nil {
					t.Errorf("Occurrence %d has nil DTStart", i)
					continue
				}
				if !occurrences[i].DTStart.Equal(expectedDate) {
					t.Errorf("Occurrence %d: got %v (%s), want %v (%s)",
						i,
						occurrences[i].DTStart.Format("2006-01-02"),
						occurrences[i].DTStart.Weekday(),
						expectedDate.Format("2006-01-02"),
						expectedDate.Weekday())
				}
			}
		})
	}
}

func TestExpandEventWithIntervalAndLimits(t *testing.T) {
	tests := []struct {
		name          string
		event         ParsedEvent
		start         time.Time
		end           time.Time
		expectedCount int
	}{
		{
			name: "Biweekly with COUNT limit",
			event: ParsedEvent{
				Summary:        "Biweekly Limited",
				DTStart:        timePtr(time.Date(2024, 9, 7, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=SA;COUNT=5",
			},
			start:         time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedCount: 5,
		},
		{
			name: "Daily INTERVAL=3 with UNTIL date",
			event: ParsedEvent{
				Summary:        "Every 3 days until Nov",
				DTStart:        timePtr(time.Date(2024, 9, 1, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=DAILY;INTERVAL=3;UNTIL=20241101T100000Z",
			},
			start:         time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:           time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
			expectedCount: 21,
		},
		{
			name: "Monthly INTERVAL=2 with EXDATE",
			event: ParsedEvent{
				Summary:        "Bimonthly with exclusion",
				DTStart:        timePtr(time.Date(2024, 9, 15, 10, 0, 0, 0, time.UTC)),
				RecurrenceRule: "FREQ=MONTHLY;INTERVAL=2",
				ExceptionDates: []time.Time{
					time.Date(2024, 11, 15, 10, 0, 0, 0, time.UTC),
				},
			},
			start:         time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
			end:           time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			occurrences, err := ExpandEvent(tt.event, tt.start, tt.end)
			if err != nil {
				t.Fatalf("ExpandEvent() error = %v", err)
			}

			if len(occurrences) != tt.expectedCount {
				t.Errorf("ExpandEvent() returned %d events, want %d", len(occurrences), tt.expectedCount)
				for i, occ := range occurrences {
					if occ.DTStart != nil {
						t.Logf("  Got[%d]: %v", i, occ.DTStart.Format("2006-01-02"))
					}
				}
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
