// Package main demonstrates connecting to an existing Tor daemon.
// This is the recommended approach for production environments where
// a system Tor daemon is already running.
//
// Production Use Case:
//
// Development vs Production:
//   - Development: Launch Tor programmatically (see simple_client example)
//   - Production: Connect to system Tor daemon (this example)
//
// Benefits of System Tor:
//   - Single Tor instance shared across all applications
//   - Tor starts on system boot, managed by systemd/init
//   - Configuration managed by system admin (firewall, logging, etc.)
//   - Better resource utilization (no duplicate processes)
//   - Persistent circuits and guard nodes across application restarts
//
// Installation:
//
//	Ubuntu/Debian: sudo apt install tor
//	Fedora/RHEL:   sudo dnf install tor
//	macOS:         brew install tor
//	Then enable:   sudo systemctl enable --now tor
//
// Default Ports:
//
//	SocksPort:   127.0.0.1:9050 (for routing traffic)
//	ControlPort: 127.0.0.1:9051 (for management, requires auth)
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Connect to system Tor daemon (default ports on Debian/Ubuntu)
	// WHY hardcoded port: System Tor uses well-known default ports.
	// Unlike examples that use :0 (random ports), production systems
	// use consistent, predictable addresses.
	//
	// NOTE: If your Tor configuration differs, adjust the address:
	//   - Custom port: "127.0.0.1:19050"
	//   - Remote Tor: "10.0.1.5:9050" (if Tor runs on another machine)
	fmt.Println("Connecting to existing Tor daemon...")

	clientCfg, err := tornago.NewClientConfig(
		// WHY no ControlPort: For simple HTTP requests, we only need SocksPort.
		// ControlPort is only needed for circuit management, Hidden Services, etc.
		tornago.WithClientSocksAddr("127.0.0.1:9050"),
		// WHY 60s timeout: Same rationale as other examples.
		// Production: Consider lower timeout (30s) if system Tor is local and fast.
		tornago.WithClientRequestTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Make a request to verify Tor connection
	// WHY check.torproject.org: Official Tor Project service that tells you
	// if your request came through Tor. Response includes your exit node IP.
	//
	// Example response: {"IsTor":true,"IP":"185.220.101.1"}
	//
	// Troubleshooting: If IsTor is false:
	//   1. Verify Tor is running: systemctl status tor
	//   2. Check SOCKS port: ss -tlnp | grep 9050
	//   3. Test manually: curl --socks5 127.0.0.1:9050 https://check.torproject.org/api/ip
	fmt.Println("Verifying Tor connection...")
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://check.torproject.org/api/ip", // Tor verification endpoint
		http.NoBody,
	)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Common errors:
		//   - "connection refused": Tor not running or wrong port
		//   - "SOCKS5 handshake failed": Tor process unhealthy
		//   - "timeout": Tor bootstrap incomplete (check logs)
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("Response from check.torproject.org:\n%s\n", string(body))
	fmt.Println("\nIf IsTor is true, your connection is successfully routed through Tor!")
}
