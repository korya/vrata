// Package vrata provides a Go implementation of localtunnel
package vrata

import (
	"context"
	"fmt"
)

// Connect creates a new tunnel with the given port and options.
// This is the main API function equivalent to the Node.js localtunnel() function.
func Connect(port int, options *TunnelOptions) (*Tunnel, error) {
	return NewTunnel(port, options)
}

// ConnectAndOpen creates and opens a tunnel in one call.
func ConnectAndOpen(port int, options *TunnelOptions) (*Tunnel, error) {
	tunnel, err := NewTunnel(port, options)
	if err != nil {
		return nil, err
	}

	if err := tunnel.Open(); err != nil {
		return nil, err
	}

	return tunnel, nil
}

// ConnectWithContext creates a tunnel with a context for cancellation.
func ConnectWithContext(ctx context.Context, port int, options *TunnelOptions) (*Tunnel, error) {
	tunnel, err := NewTunnel(port, options)
	if err != nil {
		return nil, err
	}

	// Override the tunnel's context with the provided one
	tunnel.ctx, tunnel.cancel = context.WithCancel(ctx)

	return tunnel, nil
}

// ExampleUsage demonstrates basic tunnel usage.
func ExampleUsage() {
	// Basic usage
	const defaultPort = 8080
	tunnel, err := Connect(defaultPort, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer func() { _ = tunnel.Close() }() //nolint:errcheck // best-effort cleanup in example

	// Open the tunnel
	if err := tunnel.Open(); err != nil {
		fmt.Printf("Error opening tunnel: %v\n", err)
		return
	}

	// Get the URL
	url, err := tunnel.URL()
	if err != nil {
		fmt.Printf("Error getting URL: %v\n", err)
		return
	}

	fmt.Printf("Tunnel URL: %s\n", url)

	// Listen for events
	events := tunnel.Events()
	go func() {
		for {
			select {
			case req := <-events.Request:
				fmt.Printf("Request: %s %s\n", req.Method, req.Path)
			case err := <-events.Error:
				fmt.Printf("Error: %v\n", err)
			case <-events.Close:
				fmt.Println("Tunnel closed")
				return
			}
		}
	}()
}
