package vrata

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	tunnel, err := Connect(8080, nil)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	if tunnel == nil {
		t.Error("Connect() returned nil tunnel")
		return
	}
	if tunnel.options.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", tunnel.options.Port)
	}
}

func TestConnectAndOpen(t *testing.T) {
	// Create a mock server for the tunnel registration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "test-tunnel-id",
			"url": "https://test.localtunnel.me",
			"port": 12345,
			"max_conn_count": 5
		}`))
	}))
	defer server.Close()

	// Start a local HTTP server on a random port
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello World"))
	}))
	defer localServer.Close()

	// Extract port from the local server URL
	localAddr := localServer.Listener.Addr().(*net.TCPAddr)
	localPort := localAddr.Port

	options := &TunnelOptions{
		Port: localPort,
		Host: server.URL,
	}

	tunnel, err := ConnectAndOpen(localPort, options)
	if err != nil {
		t.Fatalf("ConnectAndOpen() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	// Should have tunnel info set
	if tunnel.info == nil {
		t.Error("Tunnel info should be set after ConnectAndOpen")
	}

	// Should be able to get URL
	url, err := tunnel.URL()
	if err != nil {
		t.Fatalf("Failed to get tunnel URL: %v", err)
	}

	if url != "https://test.localtunnel.me" {
		t.Errorf("Expected URL 'https://test.localtunnel.me', got '%s'", url)
	}
}

func TestConnectWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tunnel, err := ConnectWithContext(ctx, 8080, nil)
	if err != nil {
		t.Fatalf("ConnectWithContext() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	if tunnel.ctx == nil {
		t.Error("Tunnel context should be set")
	}

	// Context should be cancelable
	done := make(chan struct{})
	go func() {
		<-tunnel.ctx.Done()
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Good, context was canceled
	case <-time.After(time.Second):
		t.Error("Context cancellation did not propagate")
	}
}

func TestConnectWithContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tunnel, err := ConnectWithContext(ctx, 8080, nil)
	if err != nil {
		t.Fatalf("ConnectWithContext() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	// Cancel the context immediately
	cancel()

	// Tunnel's context should be canceled
	select {
	case <-tunnel.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Tunnel context should be canceled when parent context is canceled")
	}
}

func TestConnectWithOptions(t *testing.T) {
	options := &TunnelOptions{
		Port:       8080,
		Host:       "https://example.com",
		Subdomain:  "test",
		LocalHost:  "127.0.0.1",
		LocalHTTPS: true,
	}

	tunnel, err := Connect(8080, options)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	// Verify options are properly set
	if tunnel.options.Host != "https://example.com" {
		t.Errorf("Expected host 'https://example.com', got '%s'", tunnel.options.Host)
	}
	if tunnel.options.Subdomain != "test" {
		t.Errorf("Expected subdomain 'test', got '%s'", tunnel.options.Subdomain)
	}
	if tunnel.options.LocalHost != "127.0.0.1" {
		t.Errorf("Expected local host '127.0.0.1', got '%s'", tunnel.options.LocalHost)
	}
	if !tunnel.options.LocalHTTPS {
		t.Error("Expected LocalHTTPS to be true")
	}
}

func TestConnectInvalidPort(t *testing.T) {
	// Port 0 should still create a tunnel (it's set in options)
	tunnel, err := Connect(0, nil)
	if err != nil {
		t.Fatalf("Connect() with port 0 failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	if tunnel.options.Port != 0 {
		t.Errorf("Expected port 0, got %d", tunnel.options.Port)
	}
}

// Add import for net package in the cluster_test.go fix.
func init() {
	// This ensures the net package is imported for cluster_test.go
}
