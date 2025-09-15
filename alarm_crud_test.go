package caldav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAddAlarmToEvent(t *testing.T) {
	tests := []struct {
		name           string
		alarm          *AlarmConfig
		eventData      string
		responseStatus int
		responseETag   string
		wantErr        bool
		expectedError  error
	}{
		{
			name: "successful display alarm",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Meeting in 15 minutes",
			},
			eventData: `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
END:VEVENT
END:VCALENDAR`,
			responseStatus: http.StatusNoContent,
			responseETag:   `"new-etag"`,
			wantErr:        false,
		},
		{
			name: "successful email alarm",
			alarm: &AlarmConfig{
				Action:      AlarmActionEmail,
				Trigger:     "-P1D",
				Description: "Meeting tomorrow",
				Summary:     "Reminder: Meeting",
				Attendees:   []string{"mailto:user@example.com"},
			},
			eventData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
END:VEVENT
END:VCALENDAR`,
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name: "successful audio alarm",
			alarm: &AlarmConfig{
				Action:  AlarmActionAudio,
				Trigger: "-PT30M",
				Attach:  "https://example.com/sound.mp3",
			},
			eventData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
END:VEVENT
END:VCALENDAR`,
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name: "alarm with repeat",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT10M",
				Description: "Meeting soon",
				Duration:    "PT5M",
				Repeat:      3,
			},
			eventData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
END:VEVENT
END:VCALENDAR`,
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name:           "nil alarm",
			alarm:          nil,
			eventData:      "BEGIN:VCALENDAR\nEND:VCALENDAR",
			responseStatus: http.StatusOK,
			wantErr:        true,
			expectedError:  fmt.Errorf("alarm configuration cannot be nil"),
		},
		{
			name: "missing action",
			alarm: &AlarmConfig{
				Trigger:     "-PT15M",
				Description: "Test",
			},
			eventData:     "BEGIN:VCALENDAR\nEND:VCALENDAR",
			wantErr:       true,
			expectedError: fmt.Errorf("alarm action is required"),
		},
		{
			name: "missing trigger",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Description: "Test",
			},
			eventData:     "BEGIN:VCALENDAR\nEND:VCALENDAR",
			wantErr:       true,
			expectedError: fmt.Errorf("alarm trigger is required"),
		},
		{
			name: "display alarm missing description",
			alarm: &AlarmConfig{
				Action:  AlarmActionDisplay,
				Trigger: "-PT15M",
			},
			eventData:     "BEGIN:VCALENDAR\nEND:VCALENDAR",
			wantErr:       true,
			expectedError: fmt.Errorf("description is required for DISPLAY alarms"),
		},
		{
			name: "email alarm missing attendees",
			alarm: &AlarmConfig{
				Action:      AlarmActionEmail,
				Trigger:     "-PT15M",
				Description: "Test",
				Summary:     "Test Summary",
				Attendees:   []string{},
			},
			eventData:     "BEGIN:VCALENDAR\nEND:VCALENDAR",
			wantErr:       true,
			expectedError: fmt.Errorf("at least one attendee is required for EMAIL alarms"),
		},
		{
			name: "repeat without duration",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Test",
				Repeat:      3,
			},
			eventData:     "BEGIN:VCALENDAR\nEND:VCALENDAR",
			wantErr:       true,
			expectedError: fmt.Errorf("duration is required when repeat is specified"),
		},
		{
			name: "event not found",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Test",
			},
			eventData:      "",
			responseStatus: http.StatusNotFound,
			wantErr:        true,
			expectedError:  &EventNotFoundError{},
		},
		{
			name: "concurrent modification",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Test",
			},
			eventData: `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR`,
			responseStatus: http.StatusPreconditionFailed,
			wantErr:        true,
			expectedError:  &ETagMismatchError{Expected: "\"test-etag\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if callCount == 1 {
					if r.Method != http.MethodGet {
						t.Errorf("expected GET request for first call, got %s", r.Method)
					}
					if tt.responseStatus == http.StatusNotFound && callCount == 1 {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.Header().Set("ETag", `"test-etag"`)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(tt.eventData))
				} else {
					if r.Method != http.MethodPut {
						t.Errorf("expected PUT request for second call, got %s", r.Method)
					}

					body, _ := io.ReadAll(r.Body)
					bodyStr := string(body)

					if tt.alarm != nil && !tt.wantErr {
						if !strings.Contains(bodyStr, "BEGIN:VALARM") {
							t.Errorf("expected VALARM in body")
						}
						if !strings.Contains(bodyStr, fmt.Sprintf("ACTION:%s", tt.alarm.Action)) {
							t.Errorf("expected ACTION:%s in body", tt.alarm.Action)
						}
						if !strings.Contains(bodyStr, fmt.Sprintf("TRIGGER:%s", tt.alarm.Trigger)) {
							t.Errorf("expected TRIGGER:%s in body", tt.alarm.Trigger)
						}
					}

					if tt.responseETag != "" {
						w.Header().Set("ETag", tt.responseETag)
					}
					w.WriteHeader(tt.responseStatus)
				}
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.SetBaseURL(server.URL)

			err := client.AddAlarmToEvent(context.Background(), "/calendars/test/event.ics", tt.alarm)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectedError != nil && !strings.Contains(err.Error(), tt.expectedError.Error()) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedError.Error(), err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRemoveAlarm(t *testing.T) {
	eventWithAlarms := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:First alarm
END:VALARM
BEGIN:VALARM
ACTION:EMAIL
TRIGGER:-PT30M
DESCRIPTION:Second alarm
SUMMARY:Email reminder
ATTENDEE:mailto:test@example.com
END:VALARM
END:VEVENT
END:VCALENDAR`

	tests := []struct {
		name           string
		alarmIndex     int
		eventData      string
		responseStatus int
		wantErr        bool
		expectedError  error
	}{
		{
			name:           "remove first alarm",
			alarmIndex:     0,
			eventData:      eventWithAlarms,
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name:           "remove second alarm",
			alarmIndex:     1,
			eventData:      eventWithAlarms,
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name:           "alarm index out of range",
			alarmIndex:     5,
			eventData:      eventWithAlarms,
			responseStatus: http.StatusOK,
			wantErr:        true,
			expectedError:  fmt.Errorf("alarm index 5 out of range"),
		},
		{
			name:           "negative alarm index",
			alarmIndex:     -1,
			eventData:      eventWithAlarms,
			responseStatus: http.StatusOK,
			wantErr:        true,
			expectedError:  fmt.Errorf("alarm index -1 out of range"),
		},
		{
			name:           "event not found",
			alarmIndex:     0,
			eventData:      "",
			responseStatus: http.StatusNotFound,
			wantErr:        true,
			expectedError:  &EventNotFoundError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if callCount == 1 {
					if r.Method != http.MethodGet {
						t.Errorf("expected GET request for first call, got %s", r.Method)
					}
					if tt.responseStatus == http.StatusNotFound && callCount == 1 {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.Header().Set("ETag", `"test-etag"`)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(tt.eventData))
				} else {
					if r.Method != http.MethodPut {
						t.Errorf("expected PUT request for second call, got %s", r.Method)
					}

					body, _ := io.ReadAll(r.Body)
					bodyStr := string(body)

					if !tt.wantErr && tt.alarmIndex == 0 {
						if strings.Contains(bodyStr, "First alarm") {
							t.Errorf("first alarm should have been removed")
						}
						if !strings.Contains(bodyStr, "Second alarm") {
							t.Errorf("second alarm should still be present")
						}
					}

					w.WriteHeader(tt.responseStatus)
				}
			}))
			defer server.Close()

			client := NewClient("test@example.com", "password")
			client.SetBaseURL(server.URL)

			err := client.RemoveAlarm(context.Background(), "/calendars/test/event.ics", tt.alarmIndex)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.expectedError != nil && !strings.Contains(err.Error(), tt.expectedError.Error()) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedError.Error(), err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRemoveAllAlarms(t *testing.T) {
	eventWithAlarms := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:First alarm
END:VALARM
BEGIN:VALARM
ACTION:EMAIL
TRIGGER:-PT30M
DESCRIPTION:Second alarm
END:VALARM
END:VEVENT
END:VCALENDAR`

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET request for first call, got %s", r.Method)
			}
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(eventWithAlarms))
		} else {
			if r.Method != http.MethodPut {
				t.Errorf("expected PUT request for second call, got %s", r.Method)
			}

			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)

			if strings.Contains(bodyStr, "BEGIN:VALARM") {
				t.Errorf("expected all alarms to be removed")
			}
			if strings.Contains(bodyStr, "ACTION:") {
				t.Errorf("expected no alarm properties")
			}

			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := NewClient("test@example.com", "password")
	client.SetBaseURL(server.URL)

	err := client.RemoveAllAlarms(context.Background(), "/calendars/test/event.ics")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAlarm(t *testing.T) {
	tests := []struct {
		name    string
		alarm   *AlarmConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid display alarm",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Test",
			},
			wantErr: false,
		},
		{
			name: "valid email alarm",
			alarm: &AlarmConfig{
				Action:      AlarmActionEmail,
				Trigger:     "-PT15M",
				Description: "Test",
				Summary:     "Test Summary",
				Attendees:   []string{"mailto:test@example.com"},
			},
			wantErr: false,
		},
		{
			name: "valid audio alarm",
			alarm: &AlarmConfig{
				Action:  AlarmActionAudio,
				Trigger: "-PT15M",
			},
			wantErr: false,
		},
		{
			name: "missing action",
			alarm: &AlarmConfig{
				Trigger: "-PT15M",
			},
			wantErr: true,
			errMsg:  "alarm action is required",
		},
		{
			name: "invalid action",
			alarm: &AlarmConfig{
				Action:  "INVALID",
				Trigger: "-PT15M",
			},
			wantErr: true,
			errMsg:  "invalid alarm action: INVALID",
		},
		{
			name: "missing trigger",
			alarm: &AlarmConfig{
				Action: AlarmActionDisplay,
			},
			wantErr: true,
			errMsg:  "alarm trigger is required",
		},
		{
			name: "display without description",
			alarm: &AlarmConfig{
				Action:  AlarmActionDisplay,
				Trigger: "-PT15M",
			},
			wantErr: true,
			errMsg:  "description is required for DISPLAY alarms",
		},
		{
			name: "email without description",
			alarm: &AlarmConfig{
				Action:    AlarmActionEmail,
				Trigger:   "-PT15M",
				Summary:   "Test",
				Attendees: []string{"test@example.com"},
			},
			wantErr: true,
			errMsg:  "description is required for EMAIL alarms",
		},
		{
			name: "email without summary",
			alarm: &AlarmConfig{
				Action:      AlarmActionEmail,
				Trigger:     "-PT15M",
				Description: "Test",
				Attendees:   []string{"test@example.com"},
			},
			wantErr: true,
			errMsg:  "summary is required for EMAIL alarms",
		},
		{
			name: "email without attendees",
			alarm: &AlarmConfig{
				Action:      AlarmActionEmail,
				Trigger:     "-PT15M",
				Description: "Test",
				Summary:     "Test",
			},
			wantErr: true,
			errMsg:  "at least one attendee is required for EMAIL alarms",
		},
		{
			name: "repeat without duration",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Test",
				Repeat:      3,
			},
			wantErr: true,
			errMsg:  "duration is required when repeat is specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAlarm(tt.alarm)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGenerateAlarmComponent(t *testing.T) {
	tests := []struct {
		name     string
		alarm    *AlarmConfig
		expected []string
	}{
		{
			name: "display alarm",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT15M",
				Description: "Meeting reminder",
			},
			expected: []string{
				"BEGIN:VALARM",
				"ACTION:DISPLAY",
				"TRIGGER:-PT15M",
				"DESCRIPTION:Meeting reminder",
				"END:VALARM",
			},
		},
		{
			name: "email alarm with attendees",
			alarm: &AlarmConfig{
				Action:      AlarmActionEmail,
				Trigger:     "-P1D",
				Description: "Meeting tomorrow",
				Summary:     "Reminder",
				Attendees:   []string{"mailto:user1@example.com", "mailto:user2@example.com"},
			},
			expected: []string{
				"BEGIN:VALARM",
				"ACTION:EMAIL",
				"TRIGGER:-P1D",
				"DESCRIPTION:Meeting tomorrow",
				"SUMMARY:Reminder",
				"ATTENDEE:mailto:user1@example.com",
				"ATTENDEE:mailto:user2@example.com",
				"END:VALARM",
			},
		},
		{
			name: "audio alarm with attachment",
			alarm: &AlarmConfig{
				Action:  AlarmActionAudio,
				Trigger: "-PT30M",
				Attach:  "https://example.com/sound.mp3",
			},
			expected: []string{
				"BEGIN:VALARM",
				"ACTION:AUDIO",
				"TRIGGER:-PT30M",
				"ATTACH:https://example.com/sound.mp3",
				"END:VALARM",
			},
		},
		{
			name: "alarm with repeat",
			alarm: &AlarmConfig{
				Action:      AlarmActionDisplay,
				Trigger:     "-PT10M",
				Description: "Repeated reminder",
				Duration:    "PT5M",
				Repeat:      3,
			},
			expected: []string{
				"BEGIN:VALARM",
				"ACTION:DISPLAY",
				"TRIGGER:-PT10M",
				"DESCRIPTION:Repeated reminder",
				"DURATION:PT5M",
				"REPEAT:3",
				"END:VALARM",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateAlarmComponent(tt.alarm)

			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("expected '%s' in alarm component", expected)
				}
			}
		})
	}
}

func TestParseAlarmTrigger(t *testing.T) {
	tests := []struct {
		name         string
		trigger      string
		wantDuration time.Duration
		wantNegative bool
		wantErr      bool
	}{
		{
			name:         "15 minutes before",
			trigger:      "-PT15M",
			wantDuration: -15 * time.Minute,
			wantNegative: true,
			wantErr:      false,
		},
		{
			name:         "1 hour before",
			trigger:      "-PT1H",
			wantDuration: -1 * time.Hour,
			wantNegative: true,
			wantErr:      false,
		},
		{
			name:         "1 day before",
			trigger:      "-P1D",
			wantDuration: -24 * time.Hour,
			wantNegative: true,
			wantErr:      false,
		},
		{
			name:         "30 minutes after",
			trigger:      "PT30M",
			wantDuration: 30 * time.Minute,
			wantNegative: false,
			wantErr:      false,
		},
		{
			name:         "complex duration",
			trigger:      "-P1DT2H30M",
			wantDuration: -(26*time.Hour + 30*time.Minute),
			wantNegative: true,
			wantErr:      false,
		},
		{
			name:         "absolute time",
			trigger:      "20240115T100000Z",
			wantDuration: 0,
			wantNegative: false,
			wantErr:      false,
		},
		{
			name:    "invalid format",
			trigger: "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, negative, err := ParseAlarmTrigger(tt.trigger)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if duration != tt.wantDuration {
					t.Errorf("expected duration %v, got %v", tt.wantDuration, duration)
				}
				if negative != tt.wantNegative {
					t.Errorf("expected negative %v, got %v", tt.wantNegative, negative)
				}
			}
		})
	}
}

func TestCreateRelativeAlarmTrigger(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		before   bool
		expected string
	}{
		{
			name:     "15 minutes before",
			duration: 15 * time.Minute,
			before:   true,
			expected: "-PT15M",
		},
		{
			name:     "1 hour before",
			duration: time.Hour,
			before:   true,
			expected: "-PT1H",
		},
		{
			name:     "1 day before",
			duration: 24 * time.Hour,
			before:   true,
			expected: "-P1D",
		},
		{
			name:     "30 minutes after",
			duration: 30 * time.Minute,
			before:   false,
			expected: "PT30M",
		},
		{
			name:     "complex duration",
			duration: 26*time.Hour + 30*time.Minute,
			before:   true,
			expected: "-P1DT2H30M",
		},
		{
			name:     "zero duration",
			duration: 0,
			before:   true,
			expected: "-PT0S",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateRelativeAlarmTrigger(tt.duration, tt.before)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCreateAbsoluteAlarmTrigger(t *testing.T) {
	testTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	expected := "20240115T100000Z"

	result := CreateAbsoluteAlarmTrigger(testTime)
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInsertAlarmIntoEvent(t *testing.T) {
	eventData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
DTSTART:20240115T100000Z
END:VEVENT
END:VCALENDAR`

	alarmComponent := `BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:Test alarm
END:VALARM
`

	result := insertAlarmIntoEvent(eventData, alarmComponent)

	if !strings.Contains(result, "BEGIN:VALARM") {
		t.Errorf("expected alarm to be inserted")
	}
	if !strings.Contains(result, "ACTION:DISPLAY") {
		t.Errorf("expected alarm action")
	}
	if !strings.Contains(result, "END:VEVENT") {
		t.Errorf("expected event end tag")
	}

	lines := strings.Split(result, "\r\n")
	alarmIndex := -1
	eventEndIndex := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "BEGIN:VALARM") {
			alarmIndex = i
		}
		if strings.HasPrefix(line, "END:VEVENT") {
			eventEndIndex = i
		}
	}

	if alarmIndex >= eventEndIndex || alarmIndex == -1 {
		t.Errorf("alarm should be inserted before END:VEVENT")
	}
}

func TestRemoveAlarmFromEvent(t *testing.T) {
	eventData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:First alarm
END:VALARM
BEGIN:VALARM
ACTION:EMAIL
TRIGGER:-PT30M
DESCRIPTION:Second alarm
END:VALARM
END:VEVENT
END:VCALENDAR`

	result := removeAlarmFromEvent(eventData, 0)

	if strings.Contains(result, "First alarm") {
		t.Errorf("first alarm should be removed")
	}
	if !strings.Contains(result, "Second alarm") {
		t.Errorf("second alarm should remain")
	}
}

func TestRemoveAllAlarmsFromEvent(t *testing.T) {
	eventData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event
SUMMARY:Test Event
BEGIN:VALARM
ACTION:DISPLAY
TRIGGER:-PT15M
DESCRIPTION:First alarm
END:VALARM
BEGIN:VALARM
ACTION:EMAIL
TRIGGER:-PT30M
DESCRIPTION:Second alarm
END:VALARM
END:VEVENT
END:VCALENDAR`

	result := removeAllAlarmsFromEvent(eventData)

	if strings.Contains(result, "BEGIN:VALARM") {
		t.Errorf("all alarms should be removed")
	}
	if strings.Contains(result, "ACTION:") {
		t.Errorf("no alarm properties should remain")
	}
	if !strings.Contains(result, "BEGIN:VEVENT") {
		t.Errorf("event should remain")
	}
	if !strings.Contains(result, "SUMMARY:Test Event") {
		t.Errorf("event properties should remain")
	}
}
