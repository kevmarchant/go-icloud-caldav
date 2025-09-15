package caldav

import (
	"net"
	"net/http"
	"time"
)

const (
	defaultMaxIdleConns        = 200
	defaultMaxIdleConnsPerHost = 20
	defaultIdleConnTimeout     = 300 * time.Second
	defaultDialTimeout         = 30 * time.Second
	defaultKeepAlive           = 300 * time.Second
	defaultTLSHandshakeTimeout = 10 * time.Second
)

type HTTPClientConfig struct {
	Timeout               time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	DisableKeepAlives     bool
	DisableCompression    bool
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
}

func DefaultHTTPClientConfig() *HTTPClientConfig {
	return &HTTPClientConfig{
		Timeout:               defaultTimeout,
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func NewOptimizedHTTPClient(config *HTTPClientConfig) *http.Client {
	if config == nil {
		config = DefaultHTTPClientConfig()
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   defaultDialTimeout,
			KeepAlive: defaultKeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,
		DisableCompression:    config.DisableCompression,
		ForceAttemptHTTP2:     true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
}

func WithOptimizedHTTPClient(config *HTTPClientConfig) ClientOption {
	return func(c *CalDAVClient) {
		c.httpClient = NewOptimizedHTTPClient(config)
	}
}

func WithConnectionPooling(maxConns, maxConnsPerHost int) ClientOption {
	return func(c *CalDAVClient) {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   defaultDialTimeout,
				KeepAlive: defaultKeepAlive,
			}).DialContext,
			MaxIdleConns:          maxConns,
			MaxIdleConnsPerHost:   maxConnsPerHost,
			IdleConnTimeout:       defaultIdleConnTimeout,
			TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
		}

		c.httpClient = &http.Client{
			Transport: transport,
			Timeout:   c.httpClient.Timeout,
		}
	}
}
