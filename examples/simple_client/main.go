// Package main provides a simple example of making HTTP requests through Tor.
//
// This example demonstrates the most basic tornago usage pattern:
//  1. Launch a Tor daemon programmatically (for development)
//  2. Create a tornago client configured to use the Tor SOCKS proxy
//  3. Make HTTP requests that automatically route through Tor
//
// Key Design Decisions:
//
// Random Ports (:0):
//   - Avoids conflicts with system Tor (typically on port 9050)
//   - Allows running multiple instances simultaneously
//   - Production alternative: Use WithTorSocksAddr("127.0.0.1:9050") to connect to system Tor
//
// 60-Second Timeout:
//   - Tor bootstrap can take 30-60 seconds on first launch
//   - Subsequent launches are faster (~5-10 seconds) due to cached consensus
//   - Production: Consider 30s for system Tor (already bootstrapped)
//
// HTTPS Target:
//   - Provides end-to-end encryption even if exit node is malicious
//   - Tor only provides anonymity, not encryption to final destination
//   - Always use HTTPS when accessing clearnet sites through Tor
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
	// Step 1: Launch Tor daemon
	// WHY: For development/testing, we launch our own Tor instance to avoid
	// requiring users to install and configure system Tor.
	fmt.Println("Starting Tor daemon...")
	launchCfg, err := tornago.NewTorLaunchConfig(
		// WHY :0 ports: Lets the OS assign random available ports.
		// This prevents conflicts with system Tor and allows multiple test instances.
		tornago.WithTorSocksAddr(":0"),   // SOCKS proxy port (for routing traffic)
		tornago.WithTorControlAddr(":0"), // ControlPort (for management commands)
		// WHY 60s timeout: Tor bootstrap downloads consensus and descriptors.
		// First launch: 30-60s, subsequent: 5-10s (cached data).
		tornago.WithTorStartupTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create launch config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor daemon: %v", err)
	}
	// WHY defer Stop(): Ensures graceful Tor shutdown even if program panics.
	// Tor will send SIGTERM, wait for cleanup, then SIGKILL if needed.
	defer torProcess.Stop()

	fmt.Printf("Tor daemon started successfully!\n")
	fmt.Printf("  SOCKS address: %s\n", torProcess.SocksAddr())
	fmt.Printf("  Control address: %s\n", torProcess.ControlAddr())

	// Step 2: Create Tor client
	// WHY separate config object: Immutable configuration pattern prevents
	// accidental modification after creation. All settings validated upfront.
	clientCfg, err := tornago.NewClientConfig(
		// WHY use torProcess.SocksAddr(): Dynamic port allocation means we
		// don't know the port until Tor starts. This gets the actual assigned port.
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		// WHY 60s request timeout: Tor circuits can be slow (3 hops of encryption).
		// Clearnet: 30s usually sufficient. Onion services: 60-90s recommended.
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

	// Step 3: Make HTTP request through Tor
	// NOTE: client.Do() is a drop-in replacement for http.Client.Do().
	// All requests automatically route through Tor with zero code changes
	// beyond client creation.
	fmt.Println("\nFetching https://example.com through Tor...")

	// WHY HTTPS: Tor encrypts traffic between relays, but exit node sees
	// plaintext HTTP. HTTPS provides end-to-end encryption, protecting
	// against malicious exit nodes.
	// Security: Always use HTTPS for sensitive data, even through Tor.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", http.NoBody)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)

	// WHY LimitReader: Prevents memory exhaustion from large responses.
	// Good practice for examples and production code.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 500))
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("\nResponse preview (first 500 bytes):\n%s\n", string(body))
	if len(body) == 500 {
		fmt.Println("... (truncated)")
	}
}
