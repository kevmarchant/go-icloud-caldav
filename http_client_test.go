package caldav

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultHTTPClientConfig(t *testing.T) {
	config := DefaultHTTPClientConfig()

	if config.Timeout != defaultTimeout {
		t.Errorf("Timeout = %v, want %v", config.Timeout, defaultTimeout)
	}
	if config.MaxIdleConns != defaultMaxIdleConns {
		t.Errorf("MaxIdleConns = %v, want %v", config.MaxIdleConns, defaultMaxIdleConns)
	}
	if config.MaxIdleConnsPerHost != defaultMaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost = %v, want %v", config.MaxIdleConnsPerHost, defaultMaxIdleConnsPerHost)
	}
	if config.IdleConnTimeout != defaultIdleConnTimeout {
		t.Errorf("IdleConnTimeout = %v, want %v", config.IdleConnTimeout, defaultIdleConnTimeout)
	}
	if config.TLSHandshakeTimeout != defaultTLSHandshakeTimeout {
		t.Errorf("TLSHandshakeTimeout = %v, want %v", config.TLSHandshakeTimeout, defaultTLSHandshakeTimeout)
	}
	if config.ExpectContinueTimeout != 1*time.Second {
		t.Errorf("ExpectContinueTimeout = %v, want %v", config.ExpectContinueTimeout, 1*time.Second)
	}
}

func TestNewOptimizedHTTPClient(t *testing.T) {
	tests := []struct {
		name   string
		config *HTTPClientConfig
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name:   "default config",
			config: DefaultHTTPClientConfig(),
		},
		{
			name: "custom config",
			config: &HTTPClientConfig{
				Timeout:               60 * time.Second,
				MaxIdleConns:          200,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       120 * time.Second,
				TLSHandshakeTimeout:   20 * time.Second,
				ExpectContinueTimeout: 2 * time.Second,
				DisableKeepAlives:     true,
				DisableCompression:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewOptimizedHTTPClient(tt.config)

			if client == nil {
				t.Fatal("NewOptimizedHTTPClient returned nil")
			}

			expectedTimeout := defaultTimeout
			if tt.config != nil && tt.config.Timeout != 0 {
				expectedTimeout = tt.config.Timeout
			}

			if client.Timeout != expectedTimeout {
				t.Errorf("client.Timeout = %v, want %v", client.Timeout, expectedTimeout)
			}

			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Fatal("Transport is not *http.Transport")
			}

			if transport == nil {
				t.Fatal("Transport is nil")
			}

			if tt.config != nil {
				if tt.config.MaxIdleConns != 0 && transport.MaxIdleConns != tt.config.MaxIdleConns {
					t.Errorf("MaxIdleConns = %v, want %v", transport.MaxIdleConns, tt.config.MaxIdleConns)
				}
				if tt.config.MaxIdleConnsPerHost != 0 && transport.MaxIdleConnsPerHost != tt.config.MaxIdleConnsPerHost {
					t.Errorf("MaxIdleConnsPerHost = %v, want %v", transport.MaxIdleConnsPerHost, tt.config.MaxIdleConnsPerHost)
				}
				if transport.DisableKeepAlives != tt.config.DisableKeepAlives {
					t.Errorf("DisableKeepAlives = %v, want %v", transport.DisableKeepAlives, tt.config.DisableKeepAlives)
				}
				if transport.DisableCompression != tt.config.DisableCompression {
					t.Errorf("DisableCompression = %v, want %v", transport.DisableCompression, tt.config.DisableCompression)
				}
			}
		})
	}
}

func TestClientOptions(t *testing.T) {
	t.Run("WithOptimizedHTTPClient", func(t *testing.T) {
		config := &HTTPClientConfig{
			Timeout:             45 * time.Second,
			MaxIdleConns:        150,
			MaxIdleConnsPerHost: 15,
		}

		client := NewClient("test@example.com", "password")
		option := WithOptimizedHTTPClient(config)
		option(client)

		if client.httpClient == nil {
			t.Fatal("httpClient is nil after applying option")
		}

		if client.httpClient.Timeout != config.Timeout {
			t.Errorf("Timeout = %v, want %v", client.httpClient.Timeout, config.Timeout)
		}
	})

	t.Run("WithConnectionPooling", func(t *testing.T) {
		maxConns := 200
		maxConnsPerHost := 20

		client := NewClient("test@example.com", "password")
		originalTimeout := client.httpClient.Timeout

		option := WithConnectionPooling(maxConns, maxConnsPerHost)
		option(client)

		if client.httpClient == nil {
			t.Fatal("httpClient is nil after applying option")
		}

		if client.httpClient.Timeout != originalTimeout {
			t.Errorf("Timeout changed unexpectedly: got %v, want %v", client.httpClient.Timeout, originalTimeout)
		}

		transport, ok := client.httpClient.Transport.(*http.Transport)
		if !ok {
			t.Fatal("Transport is not *http.Transport")
		}

		if transport.MaxIdleConns != maxConns {
			t.Errorf("MaxIdleConns = %v, want %v", transport.MaxIdleConns, maxConns)
		}

		if transport.MaxIdleConnsPerHost != maxConnsPerHost {
			t.Errorf("MaxIdleConnsPerHost = %v, want %v", transport.MaxIdleConnsPerHost, maxConnsPerHost)
		}
	})
}
