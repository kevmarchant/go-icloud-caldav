package caldav

import (
	"errors"
	"net/http"
	"testing"
)

func TestCalDAVError(t *testing.T) {
	tests := []struct {
		name       string
		err        *CalDAVError
		wantError  string
		isTemp     bool
		isAuth     bool
		isNotFound bool
	}{
		{
			name: "error with status code",
			err: &CalDAVError{
				Op:         "propfind",
				StatusCode: 404,
				Message:    "calendar not found",
			},
			wantError:  "caldav propfind: calendar not found (status 404)",
			isNotFound: true,
		},
		{
			name: "error with wrapped error",
			err: &CalDAVError{
				Op:      "report",
				Message: "failed to parse response",
				Err:     ErrInvalidXML,
			},
			wantError: "caldav report: failed to parse response: invalid XML",
		},
		{
			name: "temporary error - rate limit",
			err: &CalDAVError{
				Op:         "query",
				StatusCode: http.StatusTooManyRequests,
				Message:    "rate limited",
			},
			wantError: "caldav query: rate limited (status 429)",
			isTemp:    true,
		},
		{
			name: "temporary error - service unavailable",
			err: &CalDAVError{
				Op:         "fetch",
				StatusCode: http.StatusServiceUnavailable,
				Message:    "service unavailable",
			},
			wantError: "caldav fetch: service unavailable (status 503)",
			isTemp:    true,
		},
		{
			name: "temporary error - timeout",
			err: &CalDAVError{
				Op:      "request",
				Message: "request timed out",
				Err:     ErrTimeout,
			},
			wantError: "caldav request: request timed out: request timeout",
			isTemp:    true,
		},
		{
			name: "auth error - unauthorized",
			err: &CalDAVError{
				Op:         "authenticate",
				StatusCode: http.StatusUnauthorized,
				Message:    "invalid credentials",
			},
			wantError: "caldav authenticate: invalid credentials (status 401)",
			isAuth:    true,
		},
		{
			name: "auth error - forbidden",
			err: &CalDAVError{
				Op:         "access",
				StatusCode: http.StatusForbidden,
				Message:    "access denied",
			},
			wantError: "caldav access: access denied (status 403)",
			isAuth:    true,
		},
		{
			name: "not found error",
			err: &CalDAVError{
				Op:         "get",
				StatusCode: http.StatusNotFound,
				Message:    "resource not found",
			},
			wantError:  "caldav get: resource not found (status 404)",
			isNotFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantError {
				t.Errorf("Error() = %v, want %v", got, tt.wantError)
			}
			if got := tt.err.IsTemporary(); got != tt.isTemp {
				t.Errorf("IsTemporary() = %v, want %v", got, tt.isTemp)
			}
			if got := tt.err.IsAuthError(); got != tt.isAuth {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.isAuth)
			}
			if got := tt.err.IsNotFound(); got != tt.isNotFound {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.isNotFound)
			}
		})
	}
}

func TestCalDAVError_Unwrap(t *testing.T) {
	baseErr := errors.New("base error")
	err := &CalDAVError{
		Op:      "test",
		Message: "wrapped",
		Err:     baseErr,
	}

	if unwrapped := err.Unwrap(); unwrapped != baseErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, baseErr)
	}

	errNoWrap := &CalDAVError{
		Op:      "test",
		Message: "no wrap",
	}

	if unwrapped := errNoWrap.Unwrap(); unwrapped != nil {
		t.Errorf("Unwrap() = %v, want nil", unwrapped)
	}
}

func TestNewCalDAVError(t *testing.T) {
	err := newCalDAVError("test-op", 500, "internal error")

	if err.Op != "test-op" {
		t.Errorf("Op = %v, want test-op", err.Op)
	}
	if err.StatusCode != 500 {
		t.Errorf("StatusCode = %v, want 500", err.StatusCode)
	}
	if err.Message != "internal error" {
		t.Errorf("Message = %v, want internal error", err.Message)
	}
	if err.Err != nil {
		t.Errorf("Err = %v, want nil", err.Err)
	}
}

func TestWrapError(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		err     error
		wantNil bool
		wantOp  string
	}{
		{
			name:    "nil error returns nil",
			op:      "test",
			err:     nil,
			wantNil: true,
		},
		{
			name:   "wrap regular error",
			op:     "fetch",
			err:    errors.New("network error"),
			wantOp: "fetch",
		},
		{
			name: "already wrapped CalDAVError",
			op:   "outer",
			err: &CalDAVError{
				Op:      "inner",
				Message: "inner error",
			},
			wantOp: "inner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapError(tt.op, tt.err)

			if tt.wantNil {
				if wrapped != nil {
					t.Errorf("wrapError() = %v, want nil", wrapped)
				}
				return
			}

			if wrapped == nil {
				t.Fatal("wrapError() returned nil, want error")
			}

			if wrapped.Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", wrapped.Op, tt.wantOp)
			}
		})
	}
}

func TestMultiStatusError(t *testing.T) {
	err := &MultiStatusError{
		Op:           "batch-update",
		SuccessCount: 3,
		Errors: []error{
			errors.New("error 1"),
			errors.New("error 2"),
		},
		Responses: []ErrorResponse{
			{Href: "/cal1", StatusCode: 200},
			{Href: "/cal2", StatusCode: 200},
			{Href: "/cal3", StatusCode: 200},
			{Href: "/cal4", StatusCode: 404, Error: errors.New("not found")},
			{Href: "/cal5", StatusCode: 500, Error: errors.New("server error")},
		},
	}

	expected := "caldav batch-update: partial failure (3 succeeded, 2 failed)"
	if got := err.Error(); got != expected {
		t.Errorf("Error() = %v, want %v", got, expected)
	}

	if !err.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}

	if err.AllFailed() {
		t.Error("AllFailed() = true, want false")
	}

	allFailedErr := &MultiStatusError{
		Op:           "batch-delete",
		SuccessCount: 0,
		Errors: []error{
			errors.New("error 1"),
		},
	}

	if !allFailedErr.AllFailed() {
		t.Error("AllFailed() = false, want true for all failed case")
	}

	noErrorsErr := &MultiStatusError{
		Op:           "batch-get",
		SuccessCount: 5,
		Errors:       []error{},
	}

	if noErrorsErr.HasErrors() {
		t.Error("HasErrors() = true, want false for no errors case")
	}
}
