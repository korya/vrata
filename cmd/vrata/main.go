// Package main provides the vrata command-line tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/korya/vrata"
)

// CLI options.
var (
	port       = flag.Int("port", 0, "Internal HTTP server port")
	portShort  = flag.Int("p", 0, "Internal HTTP server port (short)")
	host       = flag.String("host", "https://localtunnel.me", "Upstream server")
	hostShort  = flag.String("h", "https://localtunnel.me", "Upstream server (short)")
	subdomain  = flag.String("subdomain", "", "Request specific subdomain")
	subShort   = flag.String("s", "", "Request specific subdomain (short)")
	localHost  = flag.String("local-host", "localhost", "Tunnel traffic to alternative localhost")
	localShort = flag.String("l", "localhost", "Tunnel traffic to alternative localhost (short)")
	localHTTPS = flag.Bool("local-https", false, "Enable HTTPS tunneling")
	open       = flag.Bool("open", false, "Automatically open tunnel URL in browser")
	openShort  = flag.Bool("o", false, "Automatically open tunnel URL in browser (short)")
	printReqs  = flag.Bool("print-requests", false, "Log request information")
	help       = flag.Bool("help", false, "Show help")
	version    = flag.Bool("version", false, "Show version")
)

// VERSION is the current version of the application.
const VERSION = "1.0.0"

func usage() {
	fmt.Fprintf(os.Stderr, `localtunnel (Go port) - Expose localhost to the world

Usage: %s [options]

Options:
  -p, --port           Internal HTTP server port (required)
  -h, --host           Upstream server (default: https://localtunnel.me)
  -s, --subdomain      Request specific subdomain
  -l, --local-host     Tunnel traffic to alternative localhost (default: localhost)
      --local-https    Enable HTTPS tunneling
  -o, --open           Automatically open tunnel URL in browser
      --print-requests Log request information
      --version        Show version
      --help           Show this help

Examples:
  %s --port 8080
  %s --port 3000 --subdomain myapp
  %s --port 8080 --open --print-requests

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *help {
		usage()
		os.Exit(0)
	}

	if *version {
		fmt.Printf("localtunnel version %s\n", VERSION)
		os.Exit(0)
	}

	// Get port from either flag
	targetPort := *port
	if targetPort == 0 {
		targetPort = *portShort
	}

	// Port is required
	if targetPort == 0 {
		// Check if port was provided as positional argument
		if len(flag.Args()) > 0 {
			if p, err := strconv.Atoi(flag.Args()[0]); err == nil {
				targetPort = p
			}
		}
	}

	if targetPort == 0 {
		fmt.Fprintf(os.Stderr, "Error: port is required\n\n")
		usage()
		os.Exit(1)
	}

	// Validate port range
	if targetPort < 1 || targetPort > 65535 {
		fmt.Fprintf(os.Stderr, "Error: port must be between 1 and 65535\n")
		os.Exit(1)
	}

	// Get other options with short flag fallbacks
	tunnelHost := *host
	if *hostShort != "https://localtunnel.me" {
		tunnelHost = *hostShort
	}

	tunnelSubdomain := *subdomain
	if *subShort != "" {
		tunnelSubdomain = *subShort
	}

	tunnelLocalHost := *localHost
	if *localShort != "localhost" {
		tunnelLocalHost = *localShort
	}

	shouldOpen := *open || *openShort

	// Create tunnel options
	options := &vrata.TunnelOptions{
		Port:       targetPort,
		Host:       tunnelHost,
		Subdomain:  tunnelSubdomain,
		LocalHost:  tunnelLocalHost,
		LocalHTTPS: *localHTTPS,
	}

	// Create tunnel
	tunnel, err := vrata.NewTunnel(targetPort, options)
	if err != nil {
		log.Fatalf("Failed to create tunnel: %v", err)
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down tunnel...")
		_ = tunnel.Close() //nolint:errcheck // best-effort shutdown
		cancel()
	}()

	// Start the tunnel
	if err := tunnel.Open(); err != nil {
		log.Fatalf("Failed to open tunnel: %v", err)
	}

	// Get the tunnel URL
	tunnelURL, err := tunnel.URL()
	if err != nil {
		log.Fatalf("Failed to get tunnel URL: %v", err)
	}

	fmt.Printf("Your tunnel is available at: %s\n", tunnelURL)

	// Open URL in browser if requested
	if shouldOpen {
		if err := vrata.OpenURL(tunnelURL); err != nil {
			fmt.Printf("Failed to open URL in browser: %v\n", err)
		}
	}

	// Handle events
	events := tunnel.Events()
	go func() {
		for {
			select {
			case req := <-events.Request:
				if *printReqs {
					// Format like localtunnel: timestamp method=GET path=/ connect=1ms service=37ms status=200 bytes=290
					fmt.Printf("%s method=%s path=%s connect=%v service=%v status=%d bytes=%d\n",
						req.Timestamp.Format("2006-01-02T15:04:05.000000Z07:00"),
						req.Method,
						req.Path,
						req.ConnectTime,
						req.ServiceTime,
						req.Status,
						req.Bytes,
					)
				}
			case err := <-events.Error:
				fmt.Printf("Tunnel error: %v\n", err)
			case <-events.Close:
				fmt.Println("Tunnel closed")
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for shutdown
	<-ctx.Done()
}
