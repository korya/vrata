package vrata

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestNewTunnelCluster(t *testing.T) {
	info := &TunnelInfo{
		ID:      "test-id",
		URL:     "https://test.localtunnel.me",
		Port:    12345,
		MaxConn: 5,
	}

	options := &TunnelOptions{
		Port:      8080,
		LocalHost: "localhost",
	}

	events := &TunnelEvents{
		URL:     make(chan string, 1),
		Error:   make(chan error, 10),
		Request: make(chan RequestInfo, 100),
		Close:   make(chan struct{}, 1),
	}

	cluster, err := NewTunnelCluster(info, options, events)
	if err != nil {
		t.Fatalf("NewTunnelCluster() failed: %v", err)
	}

	if cluster.info != info {
		t.Error("Cluster info not set correctly")
	}
	if cluster.options != options {
		t.Error("Cluster options not set correctly")
	}
	if cluster.events != events {
		t.Error("Cluster events not set correctly")
	}
}

func TestTunnelClusterClose(t *testing.T) {
	info := &TunnelInfo{
		ID:      "test-id",
		URL:     "https://test.localtunnel.me",
		Port:    12345,
		MaxConn: 5,
	}

	options := &TunnelOptions{
		Port:      8080,
		LocalHost: "localhost",
	}

	events := &TunnelEvents{
		URL:     make(chan string, 1),
		Error:   make(chan error, 10),
		Request: make(chan RequestInfo, 100),
		Close:   make(chan struct{}, 1),
	}

	cluster, err := NewTunnelCluster(info, options, events)
	if err != nil {
		t.Fatalf("NewTunnelCluster() failed: %v", err)
	}

	// Should not panic
	cluster.Close()

	// Multiple closes should be safe
	cluster.Close()
}

func TestTunnelConnection(t *testing.T) {
	cluster := &TunnelCluster{
		info: &TunnelInfo{
			ID:      "test-id",
			URL:     "https://test.localtunnel.me",
			Port:    12345,
			MaxConn: 1,
		},
		options: &TunnelOptions{
			Port:      8080,
			LocalHost: "localhost",
		},
		events: &TunnelEvents{
			URL:     make(chan string, 1),
			Error:   make(chan error, 10),
			Request: make(chan RequestInfo, 100),
			Close:   make(chan struct{}, 1),
		},
	}

	conn := &TunnelConnection{
		cluster: cluster,
	}

	// Initially should not be active
	if conn.isActive() {
		t.Error("New connection should not be active")
	}

	// Close should be safe on inactive connection
	conn.close()
}

func TestExtractRequestInfo(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected *RequestInfo
	}{
		{
			name: "valid GET request",
			data: []byte("GET /api/users HTTP/1.1\r\nHost: localhost\r\n\r\n"),
			expected: &RequestInfo{
				Method: "GET",
				Path:   "/api/users",
				URL:    "/api/users",
			},
		},
		{
			name: "valid POST request",
			data: []byte("POST /api/users HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\n\r\n"),
			expected: &RequestInfo{
				Method: "POST",
				Path:   "/api/users",
				URL:    "/api/users",
			},
		},
		{
			name:     "empty data",
			data:     []byte(""),
			expected: nil,
		},
		{
			name:     "invalid request line",
			data:     []byte("INVALID\r\n"),
			expected: nil,
		},
		{
			name:     "incomplete request line",
			data:     []byte("GET\r\n"),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRequestInfo(tt.data)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected %+v, got nil", tt.expected)
				return
			}

			if result.Method != tt.expected.Method {
				t.Errorf("Expected method %s, got %s", tt.expected.Method, result.Method)
			}
			if result.Path != tt.expected.Path {
				t.Errorf("Expected path %s, got %s", tt.expected.Path, result.Path)
			}
			if result.URL != tt.expected.URL {
				t.Errorf("Expected URL %s, got %s", tt.expected.URL, result.URL)
			}
		})
	}
}

func TestTunnelConnectionConnect(t *testing.T) {
	// Start a local TCP server for testing
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Get the actual port
	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	cluster := &TunnelCluster{
		info: &TunnelInfo{
			ID:      "test-id",
			URL:     "https://test.localtunnel.me",
			Port:    port,
			MaxConn: 1,
		},
		options: &TunnelOptions{
			Port:      8080,
			LocalHost: "localhost",
		},
		events: &TunnelEvents{
			URL:     make(chan string, 1),
			Error:   make(chan error, 10),
			Request: make(chan RequestInfo, 100),
			Close:   make(chan struct{}, 1),
		},
	}

	conn := &TunnelConnection{
		cluster: cluster,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Accept one connection
	go func() {
		testConn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = testConn.Close()
	}()

	// This should connect successfully
	conn.connect(ctx, "127.0.0.1", port)

	// Give it a moment to connect
	time.Sleep(100 * time.Millisecond)

	if !conn.isActive() {
		t.Error("Connection should be active after successful connect")
	}

	conn.close()

	if conn.isActive() {
		t.Error("Connection should not be active after close")
	}
}
