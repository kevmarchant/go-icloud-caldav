package caldav

import (
	"fmt"
	"strings"
	"time"
)

func extractDurationSign(duration string) (string, bool) {
	isNegative := strings.HasPrefix(duration, "-")
	if isNegative {
		duration = duration[1:]
	}
	return duration, isNegative
}

func validateDurationPrefix(duration string) (string, error) {
	if !strings.HasPrefix(duration, "P") {
		return "", fmt.Errorf("invalid ISO 8601 duration: %s", duration)
	}
	return duration[1:], nil
}

func processDurationComponent(numStr string, multiplier time.Duration) (time.Duration, error) {
	if numStr == "" {
		return 0, nil
	}

	num, err := parseDurationNumber(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %s", numStr)
	}

	return time.Duration(num) * multiplier, nil
}

func parseISO8601DurationSimplified(duration string) (time.Duration, bool, error) {
	duration, isNegative := extractDurationSign(duration)

	duration, err := validateDurationPrefix(duration)
	if err != nil {
		return 0, false, err
	}

	return parseDurationComponents(duration, isNegative)
}

func parseDurationComponents(duration string, isNegative bool) (time.Duration, bool, error) {
	var totalDuration time.Duration
	var numStr string

	for i, ch := range duration {
		switch ch {
		case 'D':
			d, err := processDurationComponent(numStr, 24*time.Hour)
			if err != nil {
				return 0, false, err
			}
			totalDuration += d
			numStr = ""
		case 'H':
			d, err := processDurationComponent(numStr, time.Hour)
			if err != nil {
				return 0, false, err
			}
			totalDuration += d
			numStr = ""
		case 'M':
			if i > 0 && duration[i-1] >= '0' && duration[i-1] <= '9' {
				d, err := processDurationComponent(numStr, time.Minute)
				if err != nil {
					return 0, false, err
				}
				totalDuration += d
				numStr = ""
			}
		case 'S':
			d, err := processDurationComponent(numStr, time.Second)
			if err != nil {
				return 0, false, err
			}
			totalDuration += d
			numStr = ""
		case 'T':
			continue
		default:
			if ch >= '0' && ch <= '9' {
				numStr += string(ch)
			}
		}
	}

	if isNegative {
		totalDuration = -totalDuration
	}

	return totalDuration, isNegative, nil
}
