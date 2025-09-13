package caldav

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeAuthentication
	ErrorTypeNotFound
	ErrorTypePrecondition
	ErrorTypeInvalidResponse
	ErrorTypeTimeout
	ErrorTypeCanceled
	ErrorTypeInvalidXML
	ErrorTypeNoCalendars
	ErrorTypeInvalidTimeRange
	ErrorTypeNetwork
	ErrorTypeRateLimit
	ErrorTypeServerError
	ErrorTypeInvalidRequest
	ErrorTypePermission
	ErrorTypeConflict
	ErrorTypeValidation
	ErrorTypeClient
	ErrorTypeServer
)

var (
	ErrAuthentication     = errors.New("authentication failed")
	ErrNotFound           = errors.New("resource not found")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrInvalidResponse    = errors.New("invalid server response")
	ErrTimeout            = errors.New("request timeout")
	ErrCanceled           = errors.New("request canceled")
	ErrInvalidXML         = errors.New("invalid XML")
	ErrNoCalendars        = errors.New("no calendars found")
	ErrInvalidTimeRange   = errors.New("invalid time range")
	ErrNetwork            = errors.New("network error")
	ErrRateLimit          = errors.New("rate limit exceeded")
	ErrServerError        = errors.New("server error")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrPermission         = errors.New("permission denied")
	ErrConflict           = errors.New("resource conflict")
	ErrValidation         = errors.New("validation failed")
)

type CalDAVError struct {
	Op         string
	Type       ErrorType
	StatusCode int
	Message    string
	Err        error
	Context    map[string]interface{}
}

func (e *CalDAVError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("caldav %s: %s: %v", e.Op, e.Message, e.Err)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("caldav %s: %s (status %d)", e.Op, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("caldav %s: %s", e.Op, e.Message)
}

func (e *CalDAVError) Unwrap() error {
	return e.Err
}

func (e *CalDAVError) IsTemporary() bool {
	if e.Type == ErrorTypeTimeout || e.Type == ErrorTypeRateLimit {
		return true
	}
	switch e.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusRequestTimeout:
		return true
	case 0:
		return errors.Is(e.Err, ErrTimeout)
	default:
		return false
	}
}

func (e *CalDAVError) IsAuthError() bool {
	if e.Type == ErrorTypeAuthentication || e.Type == ErrorTypePermission {
		return true
	}
	return e.StatusCode == http.StatusUnauthorized ||
		e.StatusCode == http.StatusForbidden ||
		errors.Is(e.Err, ErrAuthentication)
}

func (e *CalDAVError) IsNotFound() bool {
	if e.Type == ErrorTypeNotFound {
		return true
	}
	return e.StatusCode == http.StatusNotFound ||
		errors.Is(e.Err, ErrNotFound)
}

func (e *CalDAVError) IsNetworkError() bool {
	return e.Type == ErrorTypeNetwork || errors.Is(e.Err, ErrNetwork)
}

func (e *CalDAVError) IsValidationError() bool {
	return e.Type == ErrorTypeValidation ||
		e.Type == ErrorTypeInvalidXML ||
		e.Type == ErrorTypeInvalidTimeRange ||
		e.Type == ErrorTypeInvalidRequest ||
		errors.Is(e.Err, ErrValidation) ||
		errors.Is(e.Err, ErrInvalidXML) ||
		errors.Is(e.Err, ErrInvalidTimeRange)
}

func (e *CalDAVError) IsServerError() bool {
	if e.Type == ErrorTypeServerError {
		return true
	}
	return e.StatusCode >= 500 && e.StatusCode < 600
}

func (e *CalDAVError) IsClientError() bool {
	if e.Type == ErrorTypeInvalidRequest || e.Type == ErrorTypePrecondition {
		return true
	}
	return e.StatusCode >= 400 && e.StatusCode < 500
}

var statusCodeToErrorType = map[int]ErrorType{
	http.StatusUnauthorized:       ErrorTypeAuthentication,
	http.StatusForbidden:          ErrorTypePermission,
	http.StatusNotFound:           ErrorTypeNotFound,
	http.StatusPreconditionFailed: ErrorTypePrecondition,
	http.StatusTooManyRequests:    ErrorTypeRateLimit,
	http.StatusRequestTimeout:     ErrorTypeTimeout,
	http.StatusBadGateway:         ErrorTypeServerError,
	http.StatusGatewayTimeout:     ErrorTypeTimeout,
	http.StatusConflict:           ErrorTypeConflict,
	418:                           ErrorTypeUnknown, // I'm a teapot
}

func newCalDAVError(op string, statusCode int, message string) *CalDAVError {
	errorType := getErrorTypeFromStatus(statusCode)
	normalizedMessage := normalizeErrorMessage(message, statusCode)

	return &CalDAVError{
		Op:         op,
		Type:       errorType,
		StatusCode: statusCode,
		Message:    normalizedMessage,
	}
}

func getErrorTypeFromStatus(statusCode int) ErrorType {
	if errType, ok := statusCodeToErrorType[statusCode]; ok {
		return errType
	}

	if statusCode >= 500 {
		return ErrorTypeServerError
	} else if statusCode >= 400 {
		return ErrorTypeInvalidRequest
	}
	return ErrorTypeUnknown
}

func normalizeErrorMessage(message string, statusCode int) string {
	// Normalize message to lowercase for consistency
	normalizedMessage := strings.ToLower(message)
	if statusCode == 418 && !strings.Contains(normalizedMessage, "status") {
		normalizedMessage = fmt.Sprintf("status %d: %s", statusCode, normalizedMessage)
	}

	// Truncate long messages
	const maxMessageLength = 250
	if len(normalizedMessage) > maxMessageLength {
		normalizedMessage = normalizedMessage[:maxMessageLength] + "..."
	}

	return normalizedMessage
}

func newTypedError(op string, errorType ErrorType, message string, err error) *CalDAVError {
	return &CalDAVError{
		Op:      op,
		Type:    errorType,
		Message: message,
		Err:     err,
	}
}

func newTypedErrorWithContext(op string, errorType ErrorType, message string, err error, context map[string]interface{}) *CalDAVError {
	return &CalDAVError{
		Op:      op,
		Type:    errorType,
		Message: message,
		Err:     err,
		Context: context,
	}
}

func wrapError(op string, err error) *CalDAVError {
	return wrapErrorWithType(op, ErrorTypeUnknown, err)
}

func wrapErrorWithType(op string, errorType ErrorType, err error) *CalDAVError {
	if err == nil {
		return nil
	}

	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr
	}

	if errorType == ErrorTypeUnknown {
		errorType = inferErrorType(err)
	}

	return &CalDAVError{
		Op:      op,
		Type:    errorType,
		Message: "operation failed",
		Err:     err,
	}
}

var errorTypeMap = []struct {
	err     error
	errType ErrorType
}{
	{ErrAuthentication, ErrorTypeAuthentication},
	{ErrNotFound, ErrorTypeNotFound},
	{ErrPreconditionFailed, ErrorTypePrecondition},
	{ErrInvalidResponse, ErrorTypeInvalidResponse},
	{ErrTimeout, ErrorTypeTimeout},
	{ErrCanceled, ErrorTypeCanceled},
	{ErrInvalidXML, ErrorTypeInvalidXML},
	{ErrNoCalendars, ErrorTypeNoCalendars},
	{ErrInvalidTimeRange, ErrorTypeInvalidTimeRange},
	{ErrNetwork, ErrorTypeNetwork},
	{ErrRateLimit, ErrorTypeRateLimit},
	{ErrServerError, ErrorTypeServerError},
	{ErrInvalidRequest, ErrorTypeInvalidRequest},
	{ErrPermission, ErrorTypePermission},
	{ErrConflict, ErrorTypeConflict},
	{ErrValidation, ErrorTypeValidation},
}

func inferErrorType(err error) ErrorType {
	for _, mapping := range errorTypeMap {
		if errors.Is(err, mapping.err) {
			return mapping.errType
		}
	}
	return ErrorTypeUnknown
}

type MultiStatusError struct {
	Op           string
	SuccessCount int
	Errors       []error
	Responses    []ErrorResponse
}

type ErrorResponse struct {
	Href       string
	StatusCode int
	Error      error
}

func (e *MultiStatusError) Error() string {
	return fmt.Sprintf("caldav %s: partial failure (%d succeeded, %d failed)",
		e.Op, e.SuccessCount, len(e.Errors))
}

func (e *MultiStatusError) HasErrors() bool {
	return len(e.Errors) > 0
}

func (e *MultiStatusError) AllFailed() bool {
	return e.SuccessCount == 0 && len(e.Errors) > 0
}

func IsAuthError(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsAuthError()
	}
	return errors.Is(err, ErrAuthentication) || errors.Is(err, ErrPermission)
}

func IsNotFound(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsNotFound()
	}
	return errors.Is(err, ErrNotFound)
}

func IsTemporary(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsTemporary()
	}
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrRateLimit)
}

func IsNetworkError(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsNetworkError()
	}
	return errors.Is(err, ErrNetwork)
}

func IsValidationError(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsValidationError()
	}
	return errors.Is(err, ErrValidation) ||
		errors.Is(err, ErrInvalidXML) ||
		errors.Is(err, ErrInvalidTimeRange) ||
		errors.Is(err, ErrInvalidRequest)
}

func IsServerError(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsServerError()
	}
	return errors.Is(err, ErrServerError)
}

func IsClientError(err error) bool {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.IsClientError()
	}
	return errors.Is(err, ErrInvalidRequest) ||
		errors.Is(err, ErrPreconditionFailed) ||
		errors.Is(err, ErrValidation)
}

func GetErrorType(err error) ErrorType {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.Type
	}
	return inferErrorType(err)
}

func GetStatusCode(err error) int {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.StatusCode
	}
	return 0
}

func GetErrorContext(err error) map[string]interface{} {
	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr.Context
	}
	return nil
}
