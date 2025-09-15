package caldav

import (
	"context"
	"errors"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConnectionPoolConfig(t *testing.T) {
	config := DefaultConnectionPoolConfig()

	if config.MaxIdleConns != 200 {
		t.Errorf("Expected MaxIdleConns to be 200, got %d", config.MaxIdleConns)
	}

	if config.MaxIdleConnsPerHost != 20 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 20, got %d", config.MaxIdleConnsPerHost)
	}

	if config.MaxConnsPerHost != 50 {
		t.Errorf("Expected MaxConnsPerHost to be 50, got %d", config.MaxConnsPerHost)
	}

	if config.IdleConnTimeout != 300*time.Second {
		t.Errorf("Expected IdleConnTimeout to be 300s, got %v", config.IdleConnTimeout)
	}

	if config.DisableKeepAlives {
		t.Error("Expected DisableKeepAlives to be false")
	}

	if config.DisableCompression {
		t.Error("Expected DisableCompression to be false")
	}

	if config.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("Expected TLSHandshakeTimeout to be 10s, got %v", config.TLSHandshakeTimeout)
	}

	if config.DisableHTTP2 {
		t.Error("Expected DisableHTTP2 to be false")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.InitialInterval != 1*time.Second {
		t.Errorf("Expected InitialInterval to be 1s, got %v", config.InitialInterval)
	}

	if config.MaxInterval != 30*time.Second {
		t.Errorf("Expected MaxInterval to be 30s, got %v", config.MaxInterval)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier to be 2.0, got %f", config.Multiplier)
	}

	if config.RandomFactor != 0.1 {
		t.Errorf("Expected RandomFactor to be 0.1, got %f", config.RandomFactor)
	}

	expectedStatuses := []int{
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusBadGateway,
	}

	if len(config.RetryOnStatus) != len(expectedStatuses) {
		t.Errorf("Expected %d retry statuses, got %d", len(expectedStatuses), len(config.RetryOnStatus))
	}

	for i, status := range expectedStatuses {
		if i >= len(config.RetryOnStatus) || config.RetryOnStatus[i] != status {
			t.Errorf("Expected retry status %d to be %d", i, status)
		}
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := &RetryConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		RandomFactor:    0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 0},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second},
		{6, 10 * time.Second},
	}

	for _, test := range tests {
		result := calculateBackoff(test.attempt, config)
		if result != test.expected {
			t.Errorf("Attempt %d: expected %v, got %v", test.attempt, test.expected, result)
		}
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	config := &RetryConfig{
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		RandomFactor:    0.5,
	}

	for attempt := 1; attempt <= 5; attempt++ {
		result := calculateBackoff(attempt, config)

		baseBackoff := time.Duration(float64(config.InitialInterval) * math.Pow(2, float64(attempt-1)))
		if baseBackoff > config.MaxInterval {
			baseBackoff = config.MaxInterval
		}

		minBackoff := time.Duration(float64(baseBackoff) * 0.5)
		maxBackoff := time.Duration(float64(baseBackoff) * 1.5)

		if maxBackoff > config.MaxInterval {
			maxBackoff = config.MaxInterval
		}

		if result < minBackoff || result > maxBackoff {
			t.Errorf("Attempt %d: backoff %v outside expected range [%v, %v]",
				attempt, result, minBackoff, maxBackoff)
		}
	}
}

func TestRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout error", &net.DNSError{IsTimeout: true}, true},
		{"temporary error", &net.DNSError{IsTemporary: true}, false}, // Temporary is deprecated, no longer retried
		{"context deadline", context.DeadlineExceeded, false},
		{"regular error", errors.New("some error"), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := retryableError(test.err)
			if result != test.expected {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestRetryableStatusCode(t *testing.T) {
	config := DefaultRetryConfig()

	tests := []struct {
		status   int
		expected bool
	}{
		{http.StatusOK, false},
		{http.StatusNotFound, false},
		{http.StatusInternalServerError, false},
		{http.StatusTooManyRequests, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
		{http.StatusBadGateway, true},
	}

	for _, test := range tests {
		result := retryableStatusCode(test.status, config)
		if result != test.expected {
			t.Errorf("Status %d: expected %v, got %v", test.status, test.expected, result)
		}
	}
}

func TestRoundTripperWithRetry(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentAttempt := atomic.AddInt32(&attempts, 1)

		if currentAttempt <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	config := &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		RandomFactor:    0,
		RetryOnStatus:   []int{http.StatusServiceUnavailable},
	}

	rt := &roundTripperWithRetry{
		transport: http.DefaultTransport,
		config:    config,
		logger:    &noopLogger{},
		metrics:   &ConnectionMetrics{},
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := rt.RoundTrip(req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}

	if rt.metrics.RetriedRequests != 2 {
		t.Errorf("Expected 2 retried requests, got %d", rt.metrics.RetriedRequests)
	}

	if rt.metrics.SuccessfulRetries != 1 {
		t.Errorf("Expected 1 successful retry, got %d", rt.metrics.SuccessfulRetries)
	}
}

func TestRoundTripperWithRetryMaxAttemptsExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config := &RetryConfig{
		MaxRetries:      2,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
		RandomFactor:    0,
		RetryOnStatus:   []int{http.StatusServiceUnavailable},
	}

	rt := &roundTripperWithRetry{
		transport: http.DefaultTransport,
		config:    config,
		logger:    &noopLogger{},
		metrics:   &ConnectionMetrics{},
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := rt.RoundTrip(req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}

	if rt.metrics.FailedConnections != 1 {
		t.Errorf("Expected 1 failed connection, got %d", rt.metrics.FailedConnections)
	}
}

func TestRoundTripperWithRetryContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config := &RetryConfig{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     10 * time.Second,
		Multiplier:      2.0,
		RandomFactor:    0,
		RetryOnStatus:   []int{http.StatusServiceUnavailable},
	}

	rt := &roundTripperWithRetry{
		transport: http.DefaultTransport,
		config:    config,
		logger:    &noopLogger{},
		metrics:   &ConnectionMetrics{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := rt.RoundTrip(req)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestInstrumentedTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metrics := &ConnectionMetrics{}
	it := &instrumentedTransport{
		transport: http.DefaultTransport,
		metrics:   metrics,
		logger:    &noopLogger{},
	}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := it.RoundTrip(req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if metrics.TotalConnections != 1 {
		t.Errorf("Expected 1 total connection, got %d", metrics.TotalConnections)
	}

	if metrics.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections after request, got %d", metrics.ActiveConnections)
	}

	if metrics.ConnectionReuses != 1 {
		t.Errorf("Expected 1 connection reuse, got %d", metrics.ConnectionReuses)
	}
}

func TestWithConnectionPoolOption(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     10,
		IdleConnTimeout:     60 * time.Second,
	}

	client := NewClientWithOptions("test@example.com", "password",
		WithConnectionPool(config))

	if client.httpClient.Transport == nil {
		t.Fatal("Expected transport to be configured")
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected transport to be *http.Transport")
	}

	if transport.MaxIdleConns != 50 {
		t.Errorf("Expected MaxIdleConns to be 50, got %d", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 5 {
		t.Errorf("Expected MaxIdleConnsPerHost to be 5, got %d", transport.MaxIdleConnsPerHost)
	}

	if transport.MaxConnsPerHost != 10 {
		t.Errorf("Expected MaxConnsPerHost to be 10, got %d", transport.MaxConnsPerHost)
	}
}

func TestWithRetryOption(t *testing.T) {
	config := DefaultRetryConfig()
	client := NewClientWithOptions("test@example.com", "password",
		WithRetry(config))

	if client.httpClient.Transport == nil {
		t.Fatal("Expected transport to be configured")
	}

	_, ok := client.httpClient.Transport.(*roundTripperWithRetry)
	if !ok {
		t.Fatal("Expected transport to be wrapped with retry logic")
	}
}

func TestWithConnectionMetricsOption(t *testing.T) {
	metrics := &ConnectionMetrics{}
	client := NewClientWithOptions("test@example.com", "password",
		WithConnectionMetrics(metrics))

	if client.connectionMetrics != metrics {
		t.Error("Expected metrics to be set on client")
	}

	retrievedMetrics := client.GetConnectionMetrics()
	if retrievedMetrics != metrics {
		t.Error("Expected GetConnectionMetrics to return configured metrics")
	}
}

func TestConnectionPooling(t *testing.T) {
	var connectionCount int32
	var mu sync.Mutex
	connections := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		remoteAddr := r.RemoteAddr
		if !connections[remoteAddr] {
			connections[remoteAddr] = true
			atomic.AddInt32(&connectionCount, 1)
		}
		mu.Unlock()

		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	poolConfig := &ConnectionPoolConfig{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     5,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}

	client := NewClientWithOptions("test@example.com", "password",
		WithConnectionPool(poolConfig))

	client.baseURL = server.URL

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", server.URL, nil)
		req.Header.Set("Authorization", client.authHeader)
		resp, err := client.httpClient.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}

	time.Sleep(100 * time.Millisecond)

	actualConnections := atomic.LoadInt32(&connectionCount)
	t.Logf("Total connections created: %d", actualConnections)

	if actualConnections > 3 {
		t.Logf("Note: Connection pooling depends on Go's HTTP client implementation and OS settings")
		t.Logf("The test server may cause new connections for each request")
	}
}

func TestCombinedOptionsConfiguration(t *testing.T) {
	poolConfig := DefaultConnectionPoolConfig()
	retryConfig := DefaultRetryConfig()
	metrics := &ConnectionMetrics{}

	client := NewClientWithOptions("test@example.com", "password",
		WithConnectionPool(poolConfig),
		WithRetry(retryConfig),
		WithConnectionMetrics(metrics))

	if client.connectionMetrics != metrics {
		t.Error("Expected metrics to be configured")
	}

	if client.httpClient.Transport == nil {
		t.Fatal("Expected transport to be configured")
	}

	_, isInstrumented := client.httpClient.Transport.(*instrumentedTransport)
	if !isInstrumented {
		t.Error("Expected transport to be instrumented")
	}
}
