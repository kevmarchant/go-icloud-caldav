package caldav

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"
)

// ConnectionPoolConfig configures HTTP connection pooling and retry behavior.
type ConnectionPoolConfig struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	IdleConnTimeout       time.Duration
	DisableKeepAlives     bool
	DisableCompression    bool
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	ResponseHeaderTimeout time.Duration
	DisableHTTP2          bool
}

// DefaultConnectionPoolConfig returns sensible defaults for connection pooling.
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DisableHTTP2:          false,
	}
}

// RetryConfig configures retry behavior for failed requests.
type RetryConfig struct {
	MaxRetries      int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	RandomFactor    float64
	RetryOnStatus   []int
}

// DefaultRetryConfig returns sensible defaults for retry behavior.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
		RandomFactor:    0.1,
		RetryOnStatus:   []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusBadGateway},
	}
}

// ConnectionMetrics tracks connection pool statistics.
type ConnectionMetrics struct {
	TotalConnections  int64
	ActiveConnections int64
	IdleConnections   int64
	FailedConnections int64
	RetriedRequests   int64
	SuccessfulRetries int64
	ConnectionReuses  int64
	ConnectionCreates int64
}

// createTransport creates an HTTP transport with the given configuration.
func createTransport(config *ConnectionPoolConfig) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		ResponseHeaderTimeout: config.ResponseHeaderTimeout,
	}

	if !config.DisableHTTP2 {
		transport.ForceAttemptHTTP2 = true
	}

	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	return transport
}

// retryableError determines if an error should trigger a retry.
func retryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context deadline first as it shouldn't be retried
	if err == context.DeadlineExceeded {
		return false
	}

	if netErr, ok := err.(net.Error); ok {
		// Only check for timeouts as Temporary() is deprecated
		// Most network errors that should be retried are timeouts
		return netErr.Timeout()
	}

	return false
}

// retryableStatusCode determines if an HTTP status code should trigger a retry.
func retryableStatusCode(status int, config *RetryConfig) bool {
	for _, code := range config.RetryOnStatus {
		if status == code {
			return true
		}
	}
	return false
}

// calculateBackoff calculates the next retry interval using exponential backoff with jitter.
func calculateBackoff(attempt int, config *RetryConfig) time.Duration {
	if attempt == 0 {
		return 0
	}

	backoff := float64(config.InitialInterval) * math.Pow(config.Multiplier, float64(attempt-1))

	if config.RandomFactor > 0 {
		delta := backoff * config.RandomFactor
		minInterval := backoff - delta
		maxInterval := backoff + delta

		jitter := minInterval + (rand.Float64() * (maxInterval - minInterval))
		backoff = jitter
	}

	if backoff > float64(config.MaxInterval) {
		backoff = float64(config.MaxInterval)
	}

	return time.Duration(backoff)
}

// roundTripperWithRetry wraps an http.RoundTripper with retry logic.
type roundTripperWithRetry struct {
	transport http.RoundTripper
	config    *RetryConfig
	logger    Logger
	metrics   *ConnectionMetrics
}

func (rt *roundTripperWithRetry) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= rt.config.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := rt.waitForRetry(req, attempt); err != nil {
				return nil, err
			}
		}

		clonedReq, err := rt.prepareRequest(req)
		if err != nil {
			return nil, err
		}

		resp, lastErr = rt.transport.RoundTrip(clonedReq)

		if result, shouldContinue := rt.handleResponse(resp, lastErr, attempt); !shouldContinue {
			return result.resp, result.err
		}

		// Update lastErr if handleResponse modified it
		if resp != nil && lastErr == nil {
			lastErr = newTypedErrorWithContext("retry", ErrorTypeServer, "retryable status code", nil, map[string]interface{}{"status": resp.StatusCode})
		}
	}

	rt.recordFailure()

	if resp != nil {
		return resp, nil
	}

	// Otherwise return the error
	if lastErr != nil {
		return nil, newTypedError("retry", ErrorTypeNetwork, fmt.Sprintf("request failed after %d retries", rt.config.MaxRetries), lastErr)
	}

	return nil, newTypedError("retry", ErrorTypeNetwork, "request failed without error", nil)
}

type retryResult struct {
	resp *http.Response
	err  error
}

func (rt *roundTripperWithRetry) waitForRetry(req *http.Request, attempt int) error {
	interval := calculateBackoff(attempt, rt.config)
	rt.logger.Debug("Retrying request after %v (attempt %d/%d)", interval, attempt, rt.config.MaxRetries)
	if rt.metrics != nil {
		rt.metrics.RetriedRequests++
	}

	timer := time.NewTimer(interval)
	select {
	case <-timer.C:
		return nil
	case <-req.Context().Done():
		timer.Stop()
		return req.Context().Err()
	}
}

func (rt *roundTripperWithRetry) prepareRequest(req *http.Request) (*http.Request, error) {
	clonedReq := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, wrapErrorWithType("failed to get request body for retry", ErrorTypeClient, err)
		}
		clonedReq.Body = body
	}
	return clonedReq, nil
}

func (rt *roundTripperWithRetry) handleResponse(resp *http.Response, err error, attempt int) (retryResult, bool) {
	if err == nil && resp != nil {
		if !retryableStatusCode(resp.StatusCode, rt.config) {
			if attempt > 0 && rt.metrics != nil {
				rt.metrics.SuccessfulRetries++
			}
			return retryResult{resp: resp, err: nil}, false
		}

		rt.logger.Warn("Received retryable status code: %d", resp.StatusCode)
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		// Don't assign to err here - it's handled in the main RoundTrip function
	} else if err != nil && !retryableError(err) {
		rt.logger.Debug("Non-retryable error: %v", err)
		return retryResult{resp: nil, err: err}, false
	}

	return retryResult{}, true
}

func (rt *roundTripperWithRetry) recordFailure() {
	if rt.metrics != nil && rt.config.MaxRetries > 0 {
		rt.metrics.FailedConnections++
	}
}

// instrumentedTransport wraps a transport to collect metrics.
type instrumentedTransport struct {
	transport http.RoundTripper
	metrics   *ConnectionMetrics
	logger    Logger
}

func (it *instrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if it.metrics != nil {
		it.metrics.TotalConnections++
		it.metrics.ActiveConnections++
		defer func() {
			it.metrics.ActiveConnections--
		}()
	}

	start := time.Now()
	resp, err := it.transport.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		if it.metrics != nil {
			it.metrics.FailedConnections++
		}
		it.logger.Debug("Request failed after %v: %v", duration, err)
		return nil, err
	}

	it.logger.Debug("Request completed in %v with status %d", duration, resp.StatusCode)

	if resp.Header.Get("Connection") == "keep-alive" && it.metrics != nil {
		it.metrics.ConnectionReuses++
	} else if it.metrics != nil {
		it.metrics.ConnectionCreates++
	}

	return resp, nil
}
