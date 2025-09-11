package caldav

import (
	"errors"
	"fmt"
	"net/http"
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
)

type CalDAVError struct {
	Op         string
	StatusCode int
	Message    string
	Err        error
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
	return e.StatusCode == http.StatusUnauthorized ||
		e.StatusCode == http.StatusForbidden ||
		errors.Is(e.Err, ErrAuthentication)
}

func (e *CalDAVError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound ||
		errors.Is(e.Err, ErrNotFound)
}

func newCalDAVError(op string, statusCode int, message string) *CalDAVError {
	return &CalDAVError{
		Op:         op,
		StatusCode: statusCode,
		Message:    message,
	}
}

func wrapError(op string, err error) *CalDAVError {
	if err == nil {
		return nil
	}

	var calErr *CalDAVError
	if errors.As(err, &calErr) {
		return calErr
	}

	return &CalDAVError{
		Op:      op,
		Message: "operation failed",
		Err:     err,
	}
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
