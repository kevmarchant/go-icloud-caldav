package caldav

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
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
	cache             *ResponseCache
	// Sync optimization fields
	etagCache      *ETagCache
	preferDefaults *PreferHeader
	batchSize      int
	deltaStates    map[string]*DeltaSyncState
	syncMu         sync.RWMutex
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
		// Initialize sync optimization fields
		etagCache: &ETagCache{
			entries: make(map[string]*ETagEntry),
			maxAge:  15 * time.Minute,
		},
		preferDefaults: &PreferHeader{
			ReturnMinimal: true,
		},
		batchSize:   50,
		deltaStates: make(map[string]*DeltaSyncState),
		logger:      &noopLogger{},
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

// SetBaseURL sets the base URL for the CalDAV server.
func (c *CalDAVClient) SetBaseURL(url string) {
	c.baseURL = url
}

// GetBaseURL returns the base URL for the CalDAV server.
func (c *CalDAVClient) GetBaseURL() string {
	return c.baseURL
}

// GetHTTPClient returns the underlying HTTP client.
func (c *CalDAVClient) GetHTTPClient() *http.Client {
	return c.httpClient
}

// prepareRequest creates and configures an HTTP request with common headers.
func (c *CalDAVClient) prepareRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	var url string
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		url = path
	} else {
		url = c.baseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		c.logger.Error("Failed to create %s request: %v", method, err)
		return nil, wrapErrorWithType("request.create", ErrorTypeInvalidRequest, err)
	}

	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", userAgent)

	return req, nil
}

// setXMLHeaders sets common XML request headers.
func (c *CalDAVClient) setXMLHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
}

// setDepthHeader sets the Depth header for a request.
func (c *CalDAVClient) setDepthHeader(req *http.Request, depth string) {
	req.Header.Set("Depth", depth)
}

func (c *CalDAVClient) propfind(ctx context.Context, path string, depth string, body []byte) (*http.Response, error) {
	c.logger.Debug("PROPFIND %s (depth: %s)", path, depth)

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

	req, err := c.prepareRequest(ctx, "PROPFIND", path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.setXMLHeaders(req)
	c.setDepthHeader(req, depth)

	c.logRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("PROPFIND request failed: %v", err)
		return nil, wrapErrorWithType("propfind.execute", ErrorTypeNetwork, err)
	}

	c.logResponse(resp)
	c.logger.Info("PROPFIND %s completed with status %d", path, resp.StatusCode)

	return resp, nil
}

func (c *CalDAVClient) report(ctx context.Context, path string, body []byte) (*http.Response, error) {
	c.logger.Debug("REPORT %s", path)

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

	req, err := c.prepareRequest(ctx, "REPORT", path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	c.setXMLHeaders(req)
	c.setDepthHeader(req, "1")

	c.logRequest(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("REPORT request failed: %v", err)
		return nil, wrapErrorWithType("report.execute", ErrorTypeNetwork, err)
	}

	c.logResponse(resp)
	c.logger.Info("REPORT %s completed with status %d", path, resp.StatusCode)

	return resp, nil
}
