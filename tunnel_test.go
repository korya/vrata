package vrata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewTunnel(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		options *TunnelOptions
		wantErr bool
	}{
		{
			name:    "valid port with nil options",
			port:    8080,
			options: nil,
			wantErr: false,
		},
		{
			name: "valid port with options",
			port: 3000,
			options: &TunnelOptions{
				Host:      "https://example.com",
				Subdomain: "test",
				LocalHost: "127.0.0.1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tunnel, err := NewTunnel(tt.port, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewTunnel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tunnel == nil {
				t.Error("NewTunnel() returned nil tunnel")
				return
			}

			// Verify defaults are set
			if tunnel.options.Port != tt.port {
				t.Errorf("Expected port %d, got %d", tt.port, tunnel.options.Port)
			}
			if tunnel.options.Host == "" {
				t.Error("Host should have default value")
			}
			if tunnel.options.LocalHost == "" {
				t.Error("LocalHost should have default value")
			}
		})
	}
}

func TestTunnelOptions(t *testing.T) {
	tunnel, err := NewTunnel(8080, nil)
	if err != nil {
		t.Fatalf("NewTunnel() failed: %v", err)
	}

	// Test default values
	if tunnel.options.Host != "https://localtunnel.me" {
		t.Errorf("Expected default host 'https://localtunnel.me', got '%s'", tunnel.options.Host)
	}
	if tunnel.options.LocalHost != "localhost" {
		t.Errorf("Expected default local host 'localhost', got '%s'", tunnel.options.LocalHost)
	}
	if tunnel.options.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", tunnel.options.Port)
	}
}

func TestTunnelClose(t *testing.T) {
	tunnel, err := NewTunnel(8080, nil)
	if err != nil {
		t.Fatalf("NewTunnel() failed: %v", err)
	}

	// Close should not error on fresh tunnel
	err = tunnel.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Multiple closes should not error
	err = tunnel.Close()
	if err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}

func TestTunnelEvents(t *testing.T) {
	tunnel, err := NewTunnel(8080, nil)
	if err != nil {
		t.Fatalf("NewTunnel() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	events := tunnel.Events()
	if events == nil {
		t.Error("Events() returned nil")
		return
	}
	if events.URL == nil {
		t.Error("URL channel is nil")
	}
	if events.Error == nil {
		t.Error("Error channel is nil")
		return
	}
	if events.Request == nil {
		t.Error("Request channel is nil")
	}
	if events.Close == nil {
		t.Error("Close channel is nil")
	}
}

func TestRequestTunnelMockServer(t *testing.T) {
	// Create a mock server that returns tunnel info
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("new") == "" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "test-tunnel-id",
				"url": "https://test-tunnel.localtunnel.me",
				"port": 12345,
				"max_conn_count": 10
			}`))
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	// Create tunnel with mock server
	options := &TunnelOptions{
		Port: 8080,
		Host: server.URL,
	}
	tunnel, err := NewTunnel(8080, options)
	if err != nil {
		t.Fatalf("NewTunnel() failed: %v", err)
	}

	// Test requestTunnel
	info, err := tunnel.requestTunnel()
	if err != nil {
		t.Fatalf("requestTunnel() failed: %v", err)
	}

	if info.ID != "test-tunnel-id" {
		t.Errorf("Expected ID 'test-tunnel-id', got '%s'", info.ID)
	}
	if info.URL != "https://test-tunnel.localtunnel.me" {
		t.Errorf("Expected URL 'https://test-tunnel.localtunnel.me', got '%s'", info.URL)
	}
	if info.Port != 12345 {
		t.Errorf("Expected port 12345, got %d", info.Port)
	}
	if info.MaxConn != 10 {
		t.Errorf("Expected max_conn_count 10, got %d", info.MaxConn)
	}
}

func TestRequestTunnelWithSubdomain(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if subdomain is in URL path
		expectedPath := "/mysubdomain"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "test-id",
			"url": "https://mysubdomain.localtunnel.me",
			"port": 12345,
			"max_conn_count": 5
		}`))
	}))
	defer server.Close()

	options := &TunnelOptions{
		Port:      8080,
		Host:      server.URL,
		Subdomain: "mysubdomain",
	}
	tunnel, err := NewTunnel(8080, options)
	if err != nil {
		t.Fatalf("NewTunnel() failed: %v", err)
	}

	info, err := tunnel.requestTunnel()
	if err != nil {
		t.Fatalf("requestTunnel() failed: %v", err)
	}

	if info.URL != "https://mysubdomain.localtunnel.me" {
		t.Errorf("Expected subdomain URL, got '%s'", info.URL)
	}
}

func TestTunnelTimeout(t *testing.T) {
	// Create a mock server that hangs
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Second) // Longer than client timeout
	}))
	defer server.Close()

	options := &TunnelOptions{
		Port: 8080,
		Host: server.URL,
	}
	tunnel, err := NewTunnel(8080, options)
	if err != nil {
		t.Fatalf("NewTunnel() failed: %v", err)
	}

	// This should timeout
	_, err = tunnel.requestTunnel()
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestConnectAPI(t *testing.T) {
	tunnel, err := Connect(8080, nil)
	if err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	if tunnel.options.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", tunnel.options.Port)
	}
}

func TestTunnelWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tunnel, err := ConnectWithContext(ctx, 8080, nil)
	if err != nil {
		t.Fatalf("ConnectWithContext() failed: %v", err)
	}
	defer func() { _ = tunnel.Close() }()

	// Test that canceling the parent context cancels the tunnel context
	cancel()

	select {
	case <-tunnel.ctx.Done():
		// Good, tunnel context was canceled when parent was canceled
	case <-time.After(100 * time.Millisecond):
		t.Error("Tunnel context should be canceled when parent context is canceled")
	}
}

func TestHeaderHostTransformer(t *testing.T) {
	transformer := NewHeaderHostTransformer("localhost:8080")
	if transformer == nil {
		t.Fatal("NewHeaderHostTransformer() returned nil")
	}
	if transformer.host != "localhost:8080" {
		t.Errorf("Expected host 'localhost:8080', got '%s'", transformer.host)
	}
}
