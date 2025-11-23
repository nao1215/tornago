// Package main provides an example client that connects to an onion service through Tor.
//
// This example demonstrates accessing .onion (Hidden Services) addresses:
//  1. Launch a Tor daemon (same as simple_client)
//  2. Create a tornago client
//  3. Access .onion addresses (here: DuckDuckGo's onion service)
//
// Key Differences from Clearnet Access:
//
// Onion Routing vs Exit Nodes:
//   - Clearnet: Client -> Guard -> Middle -> Exit -> Target (exit sees destination)
//   - Onion: Client -> Guard -> Middle -> Rendezvous -> Target (no exit node involved)
//   - Benefit: Even the exit relay doesn't know what you're accessing
//
// Latency Considerations:
//   - Onion services use 6 hops total (3 from client + 3 from service)
//   - Expect 2-3x slower response times compared to clearnet
//   - Initial connection can take 10-30 seconds (rendezvous point negotiation)
//
// Security Properties:
//   - Both client and server are anonymous to each other
//   - No DNS resolution needed (*.onion addresses are cryptographic hashes)
//   - No exit node can snoop traffic (end-to-end encrypted within Tor)
//   - HTTPS still recommended for application-layer security
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
	// NOTE: Configuration identical to simple_client example.
	// Same Tor instance can access both clearnet and .onion addresses.
	fmt.Println("Starting Tor daemon...")
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),   // Random SOCKS port
		tornago.WithTorControlAddr(":0"), // Random ControlPort
		tornago.WithTorStartupTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create launch config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor daemon: %v", err)
	}
	defer torProcess.Stop()

	fmt.Printf("Tor daemon started successfully!\n")
	fmt.Printf("  SOCKS address: %s\n", torProcess.SocksAddr())
	fmt.Printf("  Control address: %s\n", torProcess.ControlAddr())

	// Step 2: Create Tor client
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
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

	// Step 3: Access .onion site (DuckDuckGo)
	// WHY DuckDuckGo: Well-known, stable onion service for demonstration.
	// The .onion address is a cryptographic hash of the service's public key.
	// Format: <56-char-base32>.onion (ED25519-V3 address, most secure)
	onionURL := "https://duckduckgogg42xjoc72x3sjasowoarfbgcmvfimaftt6twagswzczad.onion/"
	fmt.Printf("\nAccessing DuckDuckGo onion service: %s\n", onionURL)

	// NOTE: No code changes needed for .onion addresses!
	// tornago automatically detects .onion and routes through Tor's
	// Hidden Service protocol (no exit node involved).
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, onionURL, http.NoBody)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	// WARNING: First connection to onion service can take 10-30 seconds.
	// Tor must:
	//  1. Fetch the service's descriptor from the distributed hash table
	//  2. Build a circuit to a rendezvous point
	//  3. Wait for the service to connect to the same rendezvous point
	// Subsequent requests reuse the circuit and are much faster.
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500))
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("\nResponse preview (first 500 bytes):\n%s\n", string(body))
	if len(body) == 500 {
		fmt.Println("... (truncated)")
	}
}
