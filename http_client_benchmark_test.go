package caldav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func BenchmarkHTTPClientDefault(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/test/</D:href>
					<D:propstat>
						<D:prop></D:prop>
						<D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				</D:response>
			</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClient("user", "pass")
	client.baseURL = server.URL

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.propfind(context.Background(), "/", "0", []byte(`<D:prop xmlns:D="DAV:"></D:prop>`))
	}
}

func BenchmarkHTTPClientOptimized(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/test/</D:href>
					<D:propstat>
						<D:prop></D:prop>
						<D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				</D:response>
			</D:multistatus>`))
	}))
	defer server.Close()

	config := DefaultHTTPClientConfig()
	client := NewClientWithOptions("user", "pass", WithOptimizedHTTPClient(config))
	client.baseURL = server.URL

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.propfind(context.Background(), "/", "0", []byte(`<D:prop xmlns:D="DAV:"></D:prop>`))
	}
}

func BenchmarkHTTPClientConcurrent(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/test/</D:href>
					<D:propstat>
						<D:prop></D:prop>
						<D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				</D:response>
			</D:multistatus>`))
	}))
	defer server.Close()

	config := DefaultHTTPClientConfig()
	client := NewClientWithOptions("user", "pass", WithOptimizedHTTPClient(config))
	client.baseURL = server.URL

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = client.propfind(context.Background(), "/", "0", []byte(`<D:prop xmlns:D="DAV:"></D:prop>`))
		}
	})
}

func BenchmarkHTTPClientConnectionPooling(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/test/</D:href>
					<D:propstat>
						<D:prop></D:prop>
						<D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				</D:response>
			</D:multistatus>`))
	}))
	defer server.Close()

	client := NewClientWithOptions("user", "pass", WithConnectionPooling(100, 20))
	client.baseURL = server.URL

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.propfind(context.Background(), "/", "0", []byte(`<D:prop xmlns:D="DAV:"></D:prop>`))
	}
}

func BenchmarkHTTPClientManyConnections(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
			<D:multistatus xmlns:D="DAV:">
				<D:response>
					<D:href>/test/</D:href>
					<D:propstat>
						<D:prop></D:prop>
						<D:status>HTTP/1.1 200 OK</D:status>
					</D:propstat>
				</D:response>
			</D:multistatus>`))
	}))
	defer server.Close()

	config := DefaultHTTPClientConfig()
	client := NewClientWithOptions("user", "pass", WithOptimizedHTTPClient(config))
	client.baseURL = server.URL

	conns := 50
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(conns)

		for j := 0; j < conns; j++ {
			go func() {
				defer wg.Done()
				_, _ = client.propfind(context.Background(), "/", "0", []byte(`<D:prop xmlns:D="DAV:"></D:prop>`))
			}()
		}

		wg.Wait()
	}
}

func BenchmarkNewOptimizedHTTPClient(b *testing.B) {
	config := DefaultHTTPClientConfig()
	for i := 0; i < b.N; i++ {
		_ = NewOptimizedHTTPClient(config)
	}
}

func BenchmarkDefaultHTTPClientConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultHTTPClientConfig()
	}
}
