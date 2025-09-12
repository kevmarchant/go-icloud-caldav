package caldav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

type XMLValidator struct {
	AutoCorrect bool
	StrictMode  bool
	errors      []ValidationError
}

type ValidationError struct {
	Message     string
	Path        string
	Severity    string
	Correctable bool
}

type ValidationResult struct {
	Valid     bool
	Errors    []ValidationError
	Corrected []byte
	Warnings  []string
}

func NewXMLValidator(autoCorrect bool, strictMode bool) *XMLValidator {
	return &XMLValidator{
		AutoCorrect: autoCorrect,
		StrictMode:  strictMode,
		errors:      make([]ValidationError, 0),
	}
}

func (v *XMLValidator) ValidateCalDAVRequest(xmlData []byte) (*ValidationResult, error) {
	v.errors = make([]ValidationError, 0)
	result := &ValidationResult{
		Valid:     true,
		Errors:    make([]ValidationError, 0),
		Warnings:  make([]string, 0),
		Corrected: xmlData,
	}

	if !v.isWellFormed(xmlData) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Message:     "XML is not well-formed",
			Severity:    "error",
			Correctable: false,
		})
		return result, newTypedError("xml.validate", ErrorTypeInvalidXML, "XML is not well-formed", ErrInvalidXML)
	}

	corrected := xmlData
	if v.AutoCorrect {
		corrected = v.autoCorrectCommonIssues(xmlData)
	}

	v.validateNamespaces(corrected, result)
	v.validateCalendarQueryStructure(corrected, result)
	v.validatePropfindStructure(corrected, result)
	v.validateTimeRangeFormat(corrected, result)
	v.validateTextContent(corrected, result)

	if v.StrictMode {
		v.validateStrictRules(corrected, result)
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	result.Corrected = corrected
	return result, nil
}

func (v *XMLValidator) isWellFormed(xmlData []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(xmlData))
	for {
		if _, err := decoder.Token(); err != nil {
			return err.Error() == "EOF"
		}
	}
}

func (v *XMLValidator) autoCorrectCommonIssues(xmlData []byte) []byte {
	data := string(xmlData)

	doubleVCalendarPattern := regexp.MustCompile(`<C:comp-filter\s+name="VCALENDAR">\s*<C:comp-filter\s+name="VCALENDAR">`)
	if doubleVCalendarPattern.MatchString(data) {
		data = doubleVCalendarPattern.ReplaceAllString(data, `<C:comp-filter name="VCALENDAR">`)
	}

	incorrectPropFilterPattern := regexp.MustCompile(`<prop\s+name="([^"]+)">`)
	data = incorrectPropFilterPattern.ReplaceAllString(data, `<C:prop-filter name="$1">`)

	incorrectClosingPattern := regexp.MustCompile(`</prop>`)
	data = incorrectClosingPattern.ReplaceAllString(data, `</C:prop-filter>`)

	timeRangePattern := regexp.MustCompile(`start="(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2})"`)
	data = timeRangePattern.ReplaceAllStringFunc(data, func(match string) string {
		parts := strings.Split(match, `"`)
		if len(parts) >= 2 {
			timeStr := parts[1]
			timeStr = strings.ReplaceAll(timeStr, "-", "")
			timeStr = strings.ReplaceAll(timeStr, ":", "")
			timeStr = strings.ReplaceAll(timeStr, " ", "T")
			if !strings.HasSuffix(timeStr, "Z") {
				timeStr += "Z"
			}
			return fmt.Sprintf(`start="%s"`, timeStr)
		}
		return match
	})

	timeRangePattern = regexp.MustCompile(`end="(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2})"`)
	data = timeRangePattern.ReplaceAllStringFunc(data, func(match string) string {
		parts := strings.Split(match, `"`)
		if len(parts) >= 2 {
			timeStr := parts[1]
			timeStr = strings.ReplaceAll(timeStr, "-", "")
			timeStr = strings.ReplaceAll(timeStr, ":", "")
			timeStr = strings.ReplaceAll(timeStr, " ", "T")
			if !strings.HasSuffix(timeStr, "Z") {
				timeStr += "Z"
			}
			return fmt.Sprintf(`end="%s"`, timeStr)
		}
		return match
	})

	return []byte(data)
}

func (v *XMLValidator) validateNamespaces(xmlData []byte, result *ValidationResult) {
	data := string(xmlData)

	requiredNamespaces := map[string]string{
		"D": "DAV:",
		"C": "urn:ietf:params:xml:ns:caldav",
	}

	if strings.Contains(data, "<C:calendar-query") || strings.Contains(data, "<C:calendar-multiget") {
		for prefix, uri := range requiredNamespaces {
			namespaceDecl := fmt.Sprintf(`xmlns:%s="%s"`, prefix, uri)
			if !strings.Contains(data, namespaceDecl) {
				result.Errors = append(result.Errors, ValidationError{
					Message:     fmt.Sprintf("Missing required namespace: %s", namespaceDecl),
					Severity:    "error",
					Correctable: true,
				})
			}
		}
	}

	if strings.Contains(data, "<D:propfind") {
		if !strings.Contains(data, `xmlns:D="DAV:"`) {
			result.Errors = append(result.Errors, ValidationError{
				Message:     "Missing DAV: namespace in propfind",
				Severity:    "error",
				Correctable: true,
			})
		}
	}

	usedPrefixes := v.extractUsedPrefixes(data)
	for prefix := range usedPrefixes {
		if prefix != "" && !strings.Contains(data, fmt.Sprintf(`xmlns:%s=`, prefix)) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Prefix '%s' used but not declared", prefix))
		}
	}
}

func (v *XMLValidator) extractUsedPrefixes(data string) map[string]bool {
	prefixes := make(map[string]bool)

	elementPattern := regexp.MustCompile(`<([A-Z]+):([a-zA-Z-]+)`)
	matches := elementPattern.FindAllStringSubmatch(data, -1)
	for _, match := range matches {
		if len(match) > 1 {
			prefixes[match[1]] = true
		}
	}

	return prefixes
}

func (v *XMLValidator) validateCalendarQueryStructure(xmlData []byte, result *ValidationResult) {
	data := string(xmlData)

	if !strings.Contains(data, "<C:calendar-query") {
		return
	}

	vcalendarCount := strings.Count(data, `<C:comp-filter name="VCALENDAR">`)

	if strings.Contains(data, "<C:filter>") && vcalendarCount == 0 {
		result.Errors = append(result.Errors, ValidationError{
			Message:     "Calendar query filter must have VCALENDAR as root component",
			Path:        "C:filter",
			Severity:    "error",
			Correctable: false,
		})
	}

	doubleVCalendarPattern := regexp.MustCompile(`<C:comp-filter\s+name="VCALENDAR">\s*<C:comp-filter\s+name="VCALENDAR">`)
	if doubleVCalendarPattern.MatchString(data) {
		result.Errors = append(result.Errors, ValidationError{
			Message:     "Double-nested VCALENDAR components detected",
			Path:        "C:comp-filter",
			Severity:    "error",
			Correctable: true,
		})
	}

	validComponents := map[string]bool{
		"VCALENDAR": true,
		"VEVENT":    true,
		"VTODO":     true,
		"VJOURNAL":  true,
		"VFREEBUSY": true,
		"VTIMEZONE": true,
		"VALARM":    true,
	}

	compFilterPattern := regexp.MustCompile(`<C:comp-filter\s+name="([^"]+)"`)
	matches := compFilterPattern.FindAllStringSubmatch(data, -1)
	for _, match := range matches {
		if len(match) > 1 && !validComponents[match[1]] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Unknown component type: %s", match[1]))
		}
	}
}

func (v *XMLValidator) validatePropfindStructure(xmlData []byte, result *ValidationResult) {
	data := string(xmlData)

	if !strings.Contains(data, "<D:propfind") {
		return
	}

	if !strings.Contains(data, "<D:prop>") && !strings.Contains(data, "<D:prop/>") {
		result.Errors = append(result.Errors, ValidationError{
			Message:     "PROPFIND must contain a prop element",
			Path:        "D:propfind",
			Severity:    "error",
			Correctable: false,
		})
	}

	propfindCount := strings.Count(data, "<D:propfind")
	propfindCloseCount := strings.Count(data, "</D:propfind>")
	if propfindCount != propfindCloseCount {
		result.Errors = append(result.Errors, ValidationError{
			Message:     "Mismatched propfind tags",
			Severity:    "error",
			Correctable: false,
		})
	}
}

func (v *XMLValidator) validateTimeRangeFormat(xmlData []byte, result *ValidationResult) {
	data := string(xmlData)

	timeRangePattern := regexp.MustCompile(`<C:time-range\s+([^>]+)/>`)
	matches := timeRangePattern.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) > 1 {
			attrs := match[1]

			startPattern := regexp.MustCompile(`start="([^"]+)"`)
			endPattern := regexp.MustCompile(`end="([^"]+)"`)

			startMatch := startPattern.FindStringSubmatch(attrs)
			if len(startMatch) > 1 {
				if !v.isValidCalDAVTimeFormat(startMatch[1]) {
					result.Errors = append(result.Errors, ValidationError{
						Message:     fmt.Sprintf("Invalid time format in start attribute: %s", startMatch[1]),
						Path:        "C:time-range",
						Severity:    "error",
						Correctable: true,
					})
				}
			}

			endMatch := endPattern.FindStringSubmatch(attrs)
			if len(endMatch) > 1 {
				if !v.isValidCalDAVTimeFormat(endMatch[1]) {
					result.Errors = append(result.Errors, ValidationError{
						Message:     fmt.Sprintf("Invalid time format in end attribute: %s", endMatch[1]),
						Path:        "C:time-range",
						Severity:    "error",
						Correctable: true,
					})
				}
			}
		}
	}
}

func (v *XMLValidator) isValidCalDAVTimeFormat(timeStr string) bool {
	validPattern := regexp.MustCompile(`^\d{8}T\d{6}Z$`)
	return validPattern.MatchString(timeStr)
}

func (v *XMLValidator) validateTextContent(xmlData []byte, result *ValidationResult) {
	data := string(xmlData)

	unescapedPattern := regexp.MustCompile(`>([^<>]*[<>&][^<>]*)<`)
	matches := unescapedPattern.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) > 1 {
			content := match[1]
			if !strings.Contains(content, "&lt;") && !strings.Contains(content, "&gt;") &&
				!strings.Contains(content, "&amp;") && !strings.Contains(content, "&#") {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Potentially unescaped special characters in text content: %s", content))
			}
		}
	}
}

func (v *XMLValidator) validateStrictRules(xmlData []byte, result *ValidationResult) {
	data := string(xmlData)

	if !strings.HasPrefix(string(xmlData), `<?xml version="1.0" encoding="utf-8"?>`) &&
		!strings.HasPrefix(string(xmlData), `<?xml version="1.0" encoding="UTF-8"?>`) {
		result.Warnings = append(result.Warnings, "XML declaration should specify UTF-8 encoding")
	}

	if strings.Contains(data, "  ") {
		result.Warnings = append(result.Warnings, "XML contains unnecessary whitespace")
	}

	unclosedElements := v.findUnclosedElements(data)
	for _, elem := range unclosedElements {
		result.Errors = append(result.Errors, ValidationError{
			Message:     fmt.Sprintf("Unclosed element: %s", elem),
			Severity:    "error",
			Correctable: false,
		})
	}
}

func (v *XMLValidator) findUnclosedElements(data string) []string {
	stack := []string{}

	openTagPattern := regexp.MustCompile(`<([A-Z]+:[a-zA-Z-]+)(?:\s[^>]*)?>`)
	closeTagPattern := regexp.MustCompile(`</([A-Z]+:[a-zA-Z-]+)>`)
	selfClosingPattern := regexp.MustCompile(`<[A-Z]+:[a-zA-Z-]+(?:\s[^>]*)?/>`)

	processedData := selfClosingPattern.ReplaceAllString(data, "")

	for _, match := range openTagPattern.FindAllStringSubmatch(processedData, -1) {
		if len(match) > 1 {
			stack = append(stack, match[1])
		}
	}

	for _, match := range closeTagPattern.FindAllStringSubmatch(processedData, -1) {
		if len(match) > 1 && len(stack) > 0 {
			if stack[len(stack)-1] == match[1] {
				stack = stack[:len(stack)-1]
			}
		}
	}

	return stack
}

func ValidateAndCorrectXML(xmlData []byte, autoCorrect bool) ([]byte, error) {
	validator := NewXMLValidator(autoCorrect, false)
	result, err := validator.ValidateCalDAVRequest(xmlData)
	if err != nil {
		return xmlData, err
	}

	if !result.Valid && !autoCorrect {
		return xmlData, newTypedErrorWithContext("xml.validate", ErrorTypeInvalidXML, "XML validation failed", ErrInvalidXML, map[string]interface{}{"errors": result.Errors})
	}

	return result.Corrected, nil
}
