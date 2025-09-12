package caldav

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	userAgent      = "go-icloud-caldav/1.0"
)

// CalDAVClient provides access to iCloud CalDAV services.
// It handles authentication and HTTP communication with the CalDAV server.
type CalDAVClient struct {
	httpClient        *http.Client
	baseURL           string
	username          string
	password          string
	authHeader        string
	logger            Logger
	debugHTTP         bool
	xmlValidator      *XMLValidator
	autoCorrectXML    bool
	autoParsing       bool
	connectionMetrics *ConnectionMetrics
}

// NewClient creates a new CalDAV client for iCloud.
// The username should be your iCloud email address.
// The password should be an app-specific password generated from appleid.apple.com.
func NewClient(username, password string) *CalDAVClient {
	authString := fmt.Sprintf("%s:%s", username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	return &CalDAVClient{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL:    "https://caldav.icloud.com",
		username:   username,
		password:   password,
		authHeader: fmt.Sprintf("Basic %s", encodedAuth),
		logger:     &noopLogger{},
	}
}

// NewClientWithOptions creates a new CalDAV client with custom options.
// This allows configuration of logging, custom HTTP clients, and other settings.
func NewClientWithOptions(username, password string, opts ...ClientOption) *CalDAVClient {
	client := NewClient(username, password)
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// SetTimeout configures the HTTP client timeout for all requests.
// The default timeout is 30 seconds.
func (c *CalDAVClient) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// GetConnectionMetrics returns the current connection pool metrics.
// Returns nil if metrics collection is not enabled.
func (c *CalDAVClient) GetConnectionMetrics() *ConnectionMetrics {
	return c.connectionMetrics
}

func (c *CalDAVClient) propfind(ctx context.Context, path string, depth string, body []byte) (*http.Response, error) {
	var url string
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		url = path
	} else {
		url = c.baseURL + path
	}

	c.logger.Debug("PROPFIND %s (depth: %s)", url, depth)

	if c.xmlValidator != nil {
		result, err := c.xmlValidator.ValidateCalDAVRequest(body)
		if err != nil {
			c.logger.Error("XML validation error: %v", err)
			if !c.autoCorrectXML {
				return nil, newTypedError("validation", ErrorTypeValidation, "XML validation failed", err)
			}
		}

		if !result.Valid {
			if c.autoCorrectXML {
				c.logger.Warn("XML validation failed, using auto-corrected XML")
				body = result.Corrected
			} else {
				return nil, newTypedErrorWithContext("validation", ErrorTypeValidation, "XML validation failed", ErrInvalidXML, map[string]interface{}{"errors": result.Errors})
			}
		}

		for _, warning := range result.Warnings {
			c.logger.Warn("XML validation warning: %s", warning)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", url, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create PROPFIND request: %v", err)
		return nil, wrapErrorWithType("propfind.create", ErrorTypeInvalidRequest, err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", depth)

	c.logRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("PROPFIND request failed: %v", err)
		return nil, wrapErrorWithType("propfind.execute", ErrorTypeNetwork, err)
	}

	c.logResponse(resp)
	c.logger.Info("PROPFIND %s completed with status %d", url, resp.StatusCode)

	return resp, nil
}

func (c *CalDAVClient) report(ctx context.Context, path string, body []byte) (*http.Response, error) {
	var url string
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		url = path
	} else {
		url = c.baseURL + path
	}

	c.logger.Debug("REPORT %s", url)

	if c.xmlValidator != nil {
		result, err := c.xmlValidator.ValidateCalDAVRequest(body)
		if err != nil {
			c.logger.Error("XML validation error: %v", err)
			if !c.autoCorrectXML {
				return nil, newTypedError("validation", ErrorTypeValidation, "XML validation failed", err)
			}
		}

		if !result.Valid {
			if c.autoCorrectXML {
				c.logger.Warn("XML validation failed, using auto-corrected XML")
				body = result.Corrected
			} else {
				return nil, newTypedErrorWithContext("validation", ErrorTypeValidation, "XML validation failed", ErrInvalidXML, map[string]interface{}{"errors": result.Errors})
			}
		}

		for _, warning := range result.Warnings {
			c.logger.Warn("XML validation warning: %s", warning)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "REPORT", url, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create REPORT request: %v", err)
		return nil, wrapErrorWithType("report.create", ErrorTypeInvalidRequest, err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "1")

	c.logRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("REPORT request failed: %v", err)
		return nil, wrapErrorWithType("report.execute", ErrorTypeNetwork, err)
	}

	c.logResponse(resp)
	c.logger.Info("REPORT %s completed with status %d", url, resp.StatusCode)

	return resp, nil
}
