// Package main demonstrates Tor circuit rotation using NEWNYM signal.
// Circuit rotation is useful when you want to change your exit node IP address.
//
// Use Cases for Circuit Rotation:
//
// Web Scraping:
//   - Avoid IP-based rate limiting by rotating to new exit nodes
//   - Bypass per-IP request quotas
//   - NOTE: Respect robots.txt and Terms of Service
//
// Anonymity Enhancement:
//   - Change your exit IP periodically to reduce tracking
//   - Useful when accessing multiple sites sequentially
//   - Prevents correlation of activities based on exit IP
//
// Error Recovery:
//   - If a circuit fails or becomes slow, get a fresh one
//   - Tor automatically rebuilds circuits, but manual rotation can be faster
//
// How NEWNYM Works:
//   - Sends SIGNAL NEWNYM to Tor's ControlPort
//   - Tor marks existing circuits as "dirty" and builds new ones
//   - Subsequent requests use the new circuits with different exit nodes
//   - Rate limited: Max 1 NEWNYM per 10 seconds (Tor enforces this)
//
// WARNING: NEWNYM is NOT instant:
//   - Circuit building takes 5-10 seconds
//   - Requests during this time may use old circuits
//   - Best practice: Wait 10-15 seconds after NEWNYM before next request
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
	// Launch Tor daemon with ControlPort access
	// WHY ControlPort needed: NEWNYM is a ControlPort command, not available via SOCKS.
	fmt.Println("Starting Tor daemon...")
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"), // Required for NEWNYM
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

	fmt.Printf("Tor daemon started (SOCKS: %s, Control: %s)\n",
		torProcess.SocksAddr(), torProcess.ControlAddr())

	// Get control authentication
	auth, _, err := tornago.ControlAuthFromTor(torProcess.ControlAddr(), 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get control auth: %v", err)
	}

	// Create client
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

	// Create ControlClient for circuit management
	// NOTE: We have TWO clients:
	//   - client: For making HTTP requests through SOCKS
	//   - controlClient: For sending management commands (NEWNYM)
	controlClient, err := tornago.NewControlClient(
		torProcess.ControlAddr(),
		auth,
		30*time.Second,
	)
	if err != nil {
		log.Fatalf("Failed to create control client: %v", err)
	}
	defer controlClient.Close()

	if err := controlClient.Authenticate(); err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}

	// Check IP address before rotation
	// WHY check IP: Demonstrates that NEWNYM actually changes the exit node.
	// The IP we see is the exit node's IP, not our real IP.
	ip1, err := getCurrentIP(client)
	if err != nil {
		log.Fatalf("Failed to get IP: %v", err)
	}
	fmt.Printf("Current exit IP: %s\n", ip1)

	// Rotate circuit using NEWNYM
	// WHY NEWNYM: Tells Tor to discard existing circuits and build fresh ones.
	// This changes the exit node, giving you a different public IP for subsequent requests.
	//
	// IMPORTANT: Tor rate-limits NEWNYM to once per 10 seconds.
	// Calling it more frequently will return an error or be ignored.
	fmt.Println("\nRotating circuit (NEWNYM)...")
	if err := controlClient.NewIdentity(context.Background()); err != nil {
		log.Fatalf("Failed to rotate circuit: %v", err)
	}

	// Wait for new circuit to be established
	// WHY wait: NEWNYM only marks circuits as dirty. Tor needs time to:
	//   1. Build new circuits (typically 3-5 seconds)
	//   2. Select different guard/middle/exit nodes
	//   3. Complete TLS handshakes with each relay
	//
	// Production: Consider 10-15 seconds for reliability.
	// Development: 5 seconds usually sufficient for testing.
	//
	// Alternative: Use circuit status polling (see circuit_management example)
	fmt.Println("Waiting for new circuit...")
	time.Sleep(5 * time.Second)

	// Check IP address after rotation
	// NOTE: There's a small chance the new circuit uses the same exit node.
	// Tor doesn't guarantee a different exit node, just fresh circuits.
	// In practice, this is rare due to the large number of exit nodes.
	ip2, err := getCurrentIP(client)
	if err != nil {
		log.Fatalf("Failed to get IP: %v", err)
	}
	fmt.Printf("New exit IP: %s\n", ip2)

	if ip1 != ip2 {
		fmt.Println("\nCircuit rotation successful - IP changed")
	} else {
		// Edge case: Same exit node selected (rare but possible)
		// Or: Circuit building not complete yet (increase wait time)
		fmt.Println("\nWarning: IP did not change (may need more time or try again)")
	}
}

// getCurrentIP fetches the current exit node IP using the api.ipify.org API.
// This demonstrates that circuit rotation actually changes the visible IP address.

func getCurrentIP(client *tornago.Client) (string, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://api.ipify.org",
		http.NoBody,
	)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}
