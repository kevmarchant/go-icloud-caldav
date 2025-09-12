package caldav

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestCalDAVError(t *testing.T) {
	tests := []struct {
		name         string
		err          *CalDAVError
		wantError    string
		wantType     ErrorType
		isTemp       bool
		isAuth       bool
		isNotFound   bool
		isNetwork    bool
		isValidation bool
		isServer     bool
		isClient     bool
	}{
		{
			name: "error with status code",
			err: &CalDAVError{
				Op:         "propfind",
				Type:       ErrorTypeNotFound,
				StatusCode: 404,
				Message:    "calendar not found",
			},
			wantError:  "caldav propfind: calendar not found (status 404)",
			wantType:   ErrorTypeNotFound,
			isNotFound: true,
			isClient:   true,
		},
		{
			name: "error with wrapped error",
			err: &CalDAVError{
				Op:      "report",
				Type:    ErrorTypeInvalidXML,
				Message: "failed to parse response",
				Err:     ErrInvalidXML,
			},
			wantError:    "caldav report: failed to parse response: invalid XML",
			wantType:     ErrorTypeInvalidXML,
			isValidation: true,
		},
		{
			name: "temporary error - rate limit",
			err: &CalDAVError{
				Op:         "query",
				Type:       ErrorTypeRateLimit,
				StatusCode: http.StatusTooManyRequests,
				Message:    "rate limited",
			},
			wantError: "caldav query: rate limited (status 429)",
			wantType:  ErrorTypeRateLimit,
			isTemp:    true,
			isClient:  true,
		},
		{
			name: "temporary error - service unavailable",
			err: &CalDAVError{
				Op:         "fetch",
				Type:       ErrorTypeServerError,
				StatusCode: http.StatusServiceUnavailable,
				Message:    "service unavailable",
			},
			wantError: "caldav fetch: service unavailable (status 503)",
			wantType:  ErrorTypeServerError,
			isTemp:    true,
			isServer:  true,
		},
		{
			name: "temporary error - timeout",
			err: &CalDAVError{
				Op:      "request",
				Type:    ErrorTypeTimeout,
				Message: "request timed out",
				Err:     ErrTimeout,
			},
			wantError: "caldav request: request timed out: request timeout",
			wantType:  ErrorTypeTimeout,
			isTemp:    true,
		},
		{
			name: "auth error - unauthorized",
			err: &CalDAVError{
				Op:         "authenticate",
				Type:       ErrorTypeAuthentication,
				StatusCode: http.StatusUnauthorized,
				Message:    "invalid credentials",
			},
			wantError: "caldav authenticate: invalid credentials (status 401)",
			wantType:  ErrorTypeAuthentication,
			isAuth:    true,
			isClient:  true,
		},
		{
			name: "auth error - forbidden",
			err: &CalDAVError{
				Op:         "access",
				Type:       ErrorTypePermission,
				StatusCode: http.StatusForbidden,
				Message:    "access denied",
			},
			wantError: "caldav access: access denied (status 403)",
			wantType:  ErrorTypePermission,
			isAuth:    true,
			isClient:  true,
		},
		{
			name: "not found error",
			err: &CalDAVError{
				Op:         "get",
				Type:       ErrorTypeNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "resource not found",
			},
			wantError:  "caldav get: resource not found (status 404)",
			wantType:   ErrorTypeNotFound,
			isNotFound: true,
			isClient:   true,
		},
		{
			name: "network error",
			err: &CalDAVError{
				Op:      "connect",
				Type:    ErrorTypeNetwork,
				Message: "connection failed",
				Err:     ErrNetwork,
			},
			wantError: "caldav connect: connection failed: network error",
			wantType:  ErrorTypeNetwork,
			isNetwork: true,
		},
		{
			name: "validation error",
			err: &CalDAVError{
				Op:      "validate",
				Type:    ErrorTypeValidation,
				Message: "invalid data",
				Err:     ErrValidation,
			},
			wantError:    "caldav validate: invalid data: validation failed",
			wantType:     ErrorTypeValidation,
			isValidation: true,
		},
		{
			name: "server error with context",
			err: &CalDAVError{
				Op:         "process",
				Type:       ErrorTypeServerError,
				StatusCode: http.StatusInternalServerError,
				Message:    "internal server error",
				Context: map[string]interface{}{
					"request_id":  "12345",
					"retry_after": 30,
				},
			},
			wantError: "caldav process: internal server error (status 500)",
			wantType:  ErrorTypeServerError,
			isServer:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantError {
				t.Errorf("Error() = %v, want %v", got, tt.wantError)
			}
			if tt.err.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", tt.err.Type, tt.wantType)
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
			if got := tt.err.IsNetworkError(); got != tt.isNetwork {
				t.Errorf("IsNetworkError() = %v, want %v", got, tt.isNetwork)
			}
			if got := tt.err.IsValidationError(); got != tt.isValidation {
				t.Errorf("IsValidationError() = %v, want %v", got, tt.isValidation)
			}
			if got := tt.err.IsServerError(); got != tt.isServer {
				t.Errorf("IsServerError() = %v, want %v", got, tt.isServer)
			}
			if got := tt.err.IsClientError(); got != tt.isClient {
				t.Errorf("IsClientError() = %v, want %v", got, tt.isClient)
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

func TestHelperFunctions(t *testing.T) {
	authErr := &CalDAVError{
		Op:         "auth",
		Type:       ErrorTypeAuthentication,
		StatusCode: http.StatusUnauthorized,
		Message:    "auth failed",
	}

	notFoundErr := &CalDAVError{
		Op:         "find",
		Type:       ErrorTypeNotFound,
		StatusCode: http.StatusNotFound,
		Message:    "not found",
	}

	networkErr := &CalDAVError{
		Op:      "connect",
		Type:    ErrorTypeNetwork,
		Message: "network failed",
		Err:     ErrNetwork,
	}

	validationErr := &CalDAVError{
		Op:      "validate",
		Type:    ErrorTypeValidation,
		Message: "invalid",
		Err:     ErrValidation,
	}

	serverErr := &CalDAVError{
		Op:         "server",
		Type:       ErrorTypeServerError,
		StatusCode: http.StatusInternalServerError,
		Message:    "server error",
	}

	clientErr := &CalDAVError{
		Op:         "client",
		Type:       ErrorTypeInvalidRequest,
		StatusCode: http.StatusBadRequest,
		Message:    "bad request",
	}

	tests := []struct {
		name     string
		err      error
		testFunc func(error) bool
		want     bool
	}{
		{"IsAuthError with CalDAVError", authErr, IsAuthError, true},
		{"IsAuthError with ErrAuthentication", ErrAuthentication, IsAuthError, true},
		{"IsAuthError with other error", errors.New("other"), IsAuthError, false},
		{"IsNotFound with CalDAVError", notFoundErr, IsNotFound, true},
		{"IsNotFound with ErrNotFound", ErrNotFound, IsNotFound, true},
		{"IsNotFound with other error", errors.New("other"), IsNotFound, false},
		{"IsTemporary with timeout", ErrTimeout, IsTemporary, true},
		{"IsTemporary with rate limit", ErrRateLimit, IsTemporary, true},
		{"IsTemporary with other", errors.New("other"), IsTemporary, false},
		{"IsNetworkError with CalDAVError", networkErr, IsNetworkError, true},
		{"IsNetworkError with ErrNetwork", ErrNetwork, IsNetworkError, true},
		{"IsNetworkError with other", errors.New("other"), IsNetworkError, false},
		{"IsValidationError with CalDAVError", validationErr, IsValidationError, true},
		{"IsValidationError with ErrValidation", ErrValidation, IsValidationError, true},
		{"IsValidationError with ErrInvalidXML", ErrInvalidXML, IsValidationError, true},
		{"IsValidationError with other", errors.New("other"), IsValidationError, false},
		{"IsServerError with CalDAVError", serverErr, IsServerError, true},
		{"IsServerError with ErrServerError", ErrServerError, IsServerError, true},
		{"IsServerError with other", errors.New("other"), IsServerError, false},
		{"IsClientError with CalDAVError", clientErr, IsClientError, true},
		{"IsClientError with ErrInvalidRequest", ErrInvalidRequest, IsClientError, true},
		{"IsClientError with other", errors.New("other"), IsClientError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.testFunc(tt.err); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{
			name: "CalDAVError with type",
			err: &CalDAVError{
				Type: ErrorTypeAuthentication,
			},
			want: ErrorTypeAuthentication,
		},
		{
			name: "ErrAuthentication",
			err:  ErrAuthentication,
			want: ErrorTypeAuthentication,
		},
		{
			name: "ErrNetwork",
			err:  ErrNetwork,
			want: ErrorTypeNetwork,
		},
		{
			name: "Unknown error",
			err:  errors.New("unknown"),
			want: ErrorTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetErrorType(tt.err); got != tt.want {
				t.Errorf("GetErrorType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "CalDAVError with status",
			err: &CalDAVError{
				StatusCode: http.StatusNotFound,
			},
			want: http.StatusNotFound,
		},
		{
			name: "Non-CalDAVError",
			err:  errors.New("other"),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetStatusCode(tt.err); got != tt.want {
				t.Errorf("GetStatusCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetErrorContext(t *testing.T) {
	ctx := map[string]interface{}{
		"key": "value",
	}

	tests := []struct {
		name string
		err  error
		want map[string]interface{}
	}{
		{
			name: "CalDAVError with context",
			err: &CalDAVError{
				Context: ctx,
			},
			want: ctx,
		},
		{
			name: "Non-CalDAVError",
			err:  errors.New("other"),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetErrorContext(tt.err)
			if tt.want == nil && got != nil {
				t.Errorf("GetErrorContext() = %v, want nil", got)
			}
			if tt.want != nil && got == nil {
				t.Errorf("GetErrorContext() = nil, want %v", tt.want)
			}
			if tt.want != nil && got != nil {
				if got["key"] != tt.want["key"] {
					t.Errorf("GetErrorContext() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNewTypedError(t *testing.T) {
	baseErr := errors.New("base error")
	err := newTypedError("test-op", ErrorTypeValidation, "validation failed", baseErr)

	if err.Op != "test-op" {
		t.Errorf("Op = %v, want test-op", err.Op)
	}
	if err.Type != ErrorTypeValidation {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeValidation)
	}
	if err.Message != "validation failed" {
		t.Errorf("Message = %v, want validation failed", err.Message)
	}
	if err.Err != baseErr {
		t.Errorf("Err = %v, want %v", err.Err, baseErr)
	}
}

func TestNewTypedErrorWithContext(t *testing.T) {
	baseErr := errors.New("base error")
	ctx := map[string]interface{}{
		"field": "value",
	}
	err := newTypedErrorWithContext("test-op", ErrorTypeValidation, "validation failed", baseErr, ctx)

	if err.Op != "test-op" {
		t.Errorf("Op = %v, want test-op", err.Op)
	}
	if err.Type != ErrorTypeValidation {
		t.Errorf("Type = %v, want %v", err.Type, ErrorTypeValidation)
	}
	if err.Message != "validation failed" {
		t.Errorf("Message = %v, want validation failed", err.Message)
	}
	if err.Err != baseErr {
		t.Errorf("Err = %v, want %v", err.Err, baseErr)
	}
	if err.Context["field"] != "value" {
		t.Errorf("Context = %v, want %v", err.Context, ctx)
	}
}

func TestWrapErrorWithType(t *testing.T) {
	baseErr := errors.New("base error")

	tests := []struct {
		name     string
		op       string
		errType  ErrorType
		err      error
		wantNil  bool
		wantType ErrorType
		wantOp   string
	}{
		{
			name:    "nil error returns nil",
			op:      "test",
			errType: ErrorTypeNetwork,
			err:     nil,
			wantNil: true,
		},
		{
			name:     "wrap with explicit type",
			op:       "fetch",
			errType:  ErrorTypeNetwork,
			err:      baseErr,
			wantType: ErrorTypeNetwork,
			wantOp:   "fetch",
		},
		{
			name:     "wrap with inferred type",
			op:       "auth",
			errType:  ErrorTypeUnknown,
			err:      ErrAuthentication,
			wantType: ErrorTypeAuthentication,
			wantOp:   "auth",
		},
		{
			name:    "already wrapped CalDAVError",
			op:      "outer",
			errType: ErrorTypeNetwork,
			err: &CalDAVError{
				Op:   "inner",
				Type: ErrorTypeValidation,
			},
			wantOp:   "inner",
			wantType: ErrorTypeValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapErrorWithType(tt.op, tt.errType, tt.err)

			if tt.wantNil {
				if wrapped != nil {
					t.Errorf("wrapErrorWithType() = %v, want nil", wrapped)
				}
				return
			}

			if wrapped == nil {
				t.Fatal("wrapErrorWithType() returned nil, want error")
			}

			if wrapped.Op != tt.wantOp {
				t.Errorf("Op = %v, want %v", wrapped.Op, tt.wantOp)
			}

			if wrapped.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", wrapped.Type, tt.wantType)
			}
		})
	}
}

func TestInferErrorType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrorType
	}{
		{"ErrAuthentication", ErrAuthentication, ErrorTypeAuthentication},
		{"ErrNotFound", ErrNotFound, ErrorTypeNotFound},
		{"ErrPreconditionFailed", ErrPreconditionFailed, ErrorTypePrecondition},
		{"ErrInvalidResponse", ErrInvalidResponse, ErrorTypeInvalidResponse},
		{"ErrTimeout", ErrTimeout, ErrorTypeTimeout},
		{"ErrCanceled", ErrCanceled, ErrorTypeCanceled},
		{"ErrInvalidXML", ErrInvalidXML, ErrorTypeInvalidXML},
		{"ErrNoCalendars", ErrNoCalendars, ErrorTypeNoCalendars},
		{"ErrInvalidTimeRange", ErrInvalidTimeRange, ErrorTypeInvalidTimeRange},
		{"ErrNetwork", ErrNetwork, ErrorTypeNetwork},
		{"ErrRateLimit", ErrRateLimit, ErrorTypeRateLimit},
		{"ErrServerError", ErrServerError, ErrorTypeServerError},
		{"ErrInvalidRequest", ErrInvalidRequest, ErrorTypeInvalidRequest},
		{"ErrPermission", ErrPermission, ErrorTypePermission},
		{"ErrConflict", ErrConflict, ErrorTypeConflict},
		{"ErrValidation", ErrValidation, ErrorTypeValidation},
		{"Unknown error", errors.New("unknown"), ErrorTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferErrorType(tt.err); got != tt.want {
				t.Errorf("inferErrorType() = %v, want %v", got, tt.want)
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
func TestNewCalDAVErrorAdditionalCases(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		statusCode int
		body       string
		wantType   ErrorType
		wantMsg    string
	}{
		{
			name:       "408 timeout",
			operation:  "request",
			statusCode: 408,
			body:       "Request Timeout",
			wantType:   ErrorTypeTimeout,
			wantMsg:    "request timeout",
		},
		{
			name:       "502 bad gateway",
			operation:  "gateway",
			statusCode: 502,
			body:       "Bad Gateway",
			wantType:   ErrorTypeServerError,
			wantMsg:    "bad gateway",
		},
		{
			name:       "504 gateway timeout",
			operation:  "gateway",
			statusCode: 504,
			body:       "Gateway Timeout",
			wantType:   ErrorTypeTimeout,
			wantMsg:    "gateway timeout",
		},
		{
			name:       "418 teapot",
			operation:  "unknown",
			statusCode: 418,
			body:       "I'm a teapot",
			wantType:   ErrorTypeUnknown,
			wantMsg:    "status 418",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calErr := newCalDAVError(tt.operation, tt.statusCode, tt.body)

			if calErr.Type != tt.wantType {
				t.Errorf("Expected error type %v, got %v", tt.wantType, calErr.Type)
			}

			if !strings.Contains(calErr.Message, tt.wantMsg) {
				t.Errorf("Expected message to contain %q, got %q", tt.wantMsg, calErr.Message)
			}

			if calErr.StatusCode != tt.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.statusCode, calErr.StatusCode)
			}
		})
	}
}

func TestCalDAVErrorWithLongBody(t *testing.T) {
	longBody := strings.Repeat("error ", 100)
	calErr := newCalDAVError("test", 500, longBody)

	errStr := calErr.Error()
	if len(errStr) > 300 {
		t.Errorf("Error message too long, should be truncated: %d chars", len(errStr))
	}

	if !strings.Contains(errStr, "...") {
		t.Error("Truncated error should contain ellipsis")
	}
}
