package caldav

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

type mockLogger struct {
	debugCalls []string
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.debugCalls = append(m.debugCalls, msg)
}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.infoCalls = append(m.infoCalls, msg)
}

func (m *mockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.warnCalls = append(m.warnCalls, msg)
}

func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.errorCalls = append(m.errorCalls, msg)
}

func TestNewStandardLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		wantFunc func(*standardLogger) bool
	}{
		{
			name:  "debug level",
			level: LogLevelDebug,
			wantFunc: func(l *standardLogger) bool {
				return l.level == LogLevelDebug
			},
		},
		{
			name:  "info level",
			level: LogLevelInfo,
			wantFunc: func(l *standardLogger) bool {
				return l.level == LogLevelInfo
			},
		},
		{
			name:  "warn level",
			level: LogLevelWarn,
			wantFunc: func(l *standardLogger) bool {
				return l.level == LogLevelWarn
			},
		},
		{
			name:  "error level",
			level: LogLevelError,
			wantFunc: func(l *standardLogger) bool {
				return l.level == LogLevelError
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewStandardLogger(os.Stderr, tt.level)
			if sl, ok := logger.(*standardLogger); ok {
				if !tt.wantFunc(sl) {
					t.Errorf("NewStandardLogger() validation failed")
				}
			} else {
				t.Errorf("NewStandardLogger() returned wrong type")
			}
		})
	}
}

func TestStandardLoggerDebug(t *testing.T) {
	tests := []struct {
		name            string
		level           LogLevel
		message         string
		keysAndValues   []interface{}
		wantLogContains string
		wantLogged      bool
	}{
		{
			name:            "debug level logs debug",
			level:           LogLevelDebug,
			message:         "debug message",
			keysAndValues:   []interface{}{"key", "value"},
			wantLogContains: "[DEBUG] debug message",
			wantLogged:      true,
		},
		{
			name:          "info level skips debug",
			level:         LogLevelInfo,
			message:       "debug message",
			keysAndValues: []interface{}{"key", "value"},
			wantLogged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &standardLogger{
				logger: log.New(&buf, "", 0),
				level:  tt.level,
			}

			logger.Debug(tt.message, tt.keysAndValues...)

			logOutput := buf.String()
			if tt.wantLogged {
				if !strings.Contains(logOutput, tt.wantLogContains) {
					t.Errorf("Debug() output = %q, want contains %q", logOutput, tt.wantLogContains)
				}
			} else {
				if logOutput != "" {
					t.Errorf("Debug() output = %q, want empty", logOutput)
				}
			}
		})
	}
}

func TestStandardLoggerInfo(t *testing.T) {
	tests := []struct {
		name            string
		level           LogLevel
		message         string
		keysAndValues   []interface{}
		wantLogContains string
		wantLogged      bool
	}{
		{
			name:            "info level logs info",
			level:           LogLevelInfo,
			message:         "info message",
			keysAndValues:   []interface{}{"count", 42},
			wantLogContains: "[INFO] info message",
			wantLogged:      true,
		},
		{
			name:          "warn level skips info",
			level:         LogLevelWarn,
			message:       "info message",
			keysAndValues: []interface{}{},
			wantLogged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &standardLogger{
				logger: log.New(&buf, "", 0),
				level:  tt.level,
			}

			logger.Info(tt.message, tt.keysAndValues...)

			logOutput := buf.String()
			if tt.wantLogged {
				if !strings.Contains(logOutput, tt.wantLogContains) {
					t.Errorf("Info() output = %q, want contains %q", logOutput, tt.wantLogContains)
				}
			} else {
				if logOutput != "" {
					t.Errorf("Info() output = %q, want empty", logOutput)
				}
			}
		})
	}
}

func TestStandardLoggerWarn(t *testing.T) {
	tests := []struct {
		name            string
		level           LogLevel
		message         string
		keysAndValues   []interface{}
		wantLogContains string
		wantLogged      bool
	}{
		{
			name:            "warn level logs warn",
			level:           LogLevelWarn,
			message:         "warning message",
			keysAndValues:   []interface{}{"retry", 3},
			wantLogContains: "[WARN] warning message",
			wantLogged:      true,
		},
		{
			name:          "error level skips warn",
			level:         LogLevelError,
			message:       "warning message",
			keysAndValues: nil,
			wantLogged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &standardLogger{
				logger: log.New(&buf, "", 0),
				level:  tt.level,
			}

			logger.Warn(tt.message, tt.keysAndValues...)

			logOutput := buf.String()
			if tt.wantLogged {
				if !strings.Contains(logOutput, tt.wantLogContains) {
					t.Errorf("Warn() output = %q, want contains %q", logOutput, tt.wantLogContains)
				}
			} else {
				if logOutput != "" {
					t.Errorf("Warn() output = %q, want empty", logOutput)
				}
			}
		})
	}
}

func TestStandardLoggerError(t *testing.T) {
	tests := []struct {
		name            string
		level           LogLevel
		message         string
		keysAndValues   []interface{}
		wantLogContains string
	}{
		{
			name:            "error level logs error",
			level:           LogLevelError,
			message:         "error message",
			keysAndValues:   []interface{}{"error", "failure"},
			wantLogContains: "[ERROR] error message",
		},
		{
			name:            "debug level logs error",
			level:           LogLevelDebug,
			message:         "critical error",
			keysAndValues:   []interface{}{"code", 500},
			wantLogContains: "[ERROR] critical error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := &standardLogger{
				logger: log.New(&buf, "", 0),
				level:  tt.level,
			}

			logger.Error(tt.message, tt.keysAndValues...)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.wantLogContains) {
				t.Errorf("Error() output = %q, want contains %q", logOutput, tt.wantLogContains)
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	mock := &mockLogger{}
	opt := WithLogger(mock)

	client := &CalDAVClient{}
	opt(client)

	if client.logger != mock {
		t.Errorf("WithLogger() did not set logger correctly")
	}
}

func TestWithDebugLogging(t *testing.T) {
	var buf bytes.Buffer
	opt := WithDebugLogging(&buf)

	client := &CalDAVClient{}
	opt(client)

	if _, ok := client.logger.(*standardLogger); !ok {
		t.Errorf("WithDebugLogging() did not set standard logger")
	}
}

func TestNoopLogger(t *testing.T) {
	logger := &noopLogger{}

	// These should not panic and should do nothing
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// No way to assert output since it's a noop logger, but we're testing that it doesn't crash
}

func TestWithLoggerOptions(t *testing.T) {
	mock := &mockLogger{}
	connectionConfig := &ConnectionPoolConfig{
		MaxIdleConns:        10,
		MaxConnsPerHost:     5,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30,
	}
	retryConfig := &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     1000 * time.Millisecond,
		Multiplier:      2.0,
		RetryOnStatus:   []int{500, 502, 503, 504},
	}
	metrics := &ConnectionMetrics{}

	client := NewClientWithOptions(
		"user",
		"pass",
		WithLogger(mock),
		WithAutoParsing(),
		WithAutoCorrectXML(),
		WithStrictXMLValidation(),
		WithXMLValidation(true, true),
		WithConnectionPool(connectionConfig),
		WithRetry(retryConfig),
		WithConnectionMetrics(metrics),
	)

	if client.logger != mock {
		t.Error("Logger was not set correctly")
	}

	if !client.autoParsing {
		t.Error("Auto parsing was not enabled")
	}

	if !client.autoCorrectXML {
		t.Error("Auto correct XML was not enabled")
	}

	if client.xmlValidator == nil {
		t.Error("XML validator was not set")
	}

	if client.connectionMetrics == nil {
		t.Error("Connection metrics was not set")
	}
}
