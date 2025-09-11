package caldav

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
)

type LogLevel int

const (
	LogLevelNone LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type noopLogger struct{}

func (n *noopLogger) Debug(msg string, args ...interface{}) {}
func (n *noopLogger) Info(msg string, args ...interface{})  {}
func (n *noopLogger) Warn(msg string, args ...interface{})  {}
func (n *noopLogger) Error(msg string, args ...interface{}) {}

type standardLogger struct {
	logger *log.Logger
	level  LogLevel
}

func NewStandardLogger(w io.Writer, level LogLevel) Logger {
	return &standardLogger{
		logger: log.New(w, "[caldav] ", log.LstdFlags),
		level:  level,
	}
}

func (s *standardLogger) Debug(msg string, args ...interface{}) {
	if s.level >= LogLevelDebug {
		s.log("DEBUG", msg, args...)
	}
}

func (s *standardLogger) Info(msg string, args ...interface{}) {
	if s.level >= LogLevelInfo {
		s.log("INFO", msg, args...)
	}
}

func (s *standardLogger) Warn(msg string, args ...interface{}) {
	if s.level >= LogLevelWarn {
		s.log("WARN", msg, args...)
	}
}

func (s *standardLogger) Error(msg string, args ...interface{}) {
	if s.level >= LogLevelError {
		s.log("ERROR", msg, args...)
	}
}

func (s *standardLogger) log(level, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	s.logger.Printf("[%s] %s", level, formatted)
}

type ClientOption func(*CalDAVClient)

func WithLogger(logger Logger) ClientOption {
	return func(c *CalDAVClient) {
		c.logger = logger
	}
}

func WithDebugLogging(w io.Writer) ClientOption {
	return func(c *CalDAVClient) {
		c.logger = NewStandardLogger(w, LogLevelDebug)
		c.debugHTTP = true
	}
}

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *CalDAVClient) {
		c.httpClient = client
	}
}

func (c *CalDAVClient) logRequest(req *http.Request) {
	if c.debugHTTP && c.logger != nil {
		if dump, err := httputil.DumpRequestOut(req, true); err == nil {
			c.logger.Debug("HTTP Request:\n%s", string(dump))
		} else {
			c.logger.Error("Failed to dump request: %v", err)
		}
	} else if c.logger != nil {
		c.logger.Debug("HTTP %s %s", req.Method, req.URL.Path)
	}
}

func (c *CalDAVClient) logResponse(resp *http.Response) {
	if c.debugHTTP && c.logger != nil {
		if dump, err := httputil.DumpResponse(resp, true); err == nil {
			c.logger.Debug("HTTP Response:\n%s", string(dump))
		} else {
			c.logger.Error("Failed to dump response: %v", err)
		}
	} else if c.logger != nil {
		c.logger.Debug("HTTP Response: %d", resp.StatusCode)
	}
}
