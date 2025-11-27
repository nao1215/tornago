// Package main demonstrates the slow relay avoidance feature.
// This feature automatically tracks relay performance and blocks slow relays.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nao1215/tornago"
)

const errorResponse = "error"

func main() {
	// Example 1: Basic slow relay avoidance (recommended for most users)
	fmt.Println("=== Example 1: Basic Slow Relay Avoidance ===")
	basicExample()

	// Example 2: Custom threshold configuration
	fmt.Println("\n=== Example 2: Custom Threshold Configuration ===")
	customThresholdExample()
}

// basicExample demonstrates the simplest way to enable slow relay avoidance.
// This is the recommended approach for most users.
func basicExample() {
	// Launch Tor daemon
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"),
		tornago.WithTorStartupTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create launch config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor: %v", err)
	}
	defer torProcess.Stop()

	// Create client with slow relay avoidance enabled
	// This is all you need! The client automatically:
	// - Tracks latency and success rate for each relay
	// - Blocks slow or unreliable relays
	// - Rotates circuits when blocked relays are detected
	// - Excludes blocked relays from Tor's circuit building
	//
	// Default thresholds:
	//   - MaxLatency: 5 seconds
	//   - MinSuccessRate: 80%
	//   - BlockDuration: 30 minutes
	//   - MinSamples: 3
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientControlAddr(torProcess.ControlAddr()),
		tornago.WithClientRequestTimeout(60*time.Second),
		tornago.WithSlowRelayAvoidance(), // Enable with defaults
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Make some requests - performance tracking happens automatically
	fmt.Println("Making requests through Tor...")
	for i := range 5 {
		exitIP := getExitIP(client)
		fmt.Printf("Request %d - Exit IP: %s\n", i+1, exitIP)
	}

	// Check relay performance statistics
	stats, ok := client.RelayPerformanceStats()
	if ok {
		fmt.Printf("\nRelay Performance Statistics:\n")
		fmt.Printf("  Enabled: %v\n", stats.Enabled())
		fmt.Printf("  Tracked relays: %d\n", stats.TrackedRelays())
		fmt.Printf("  Blocked relays: %d\n", stats.BlockedRelays())
		fmt.Printf("  Threshold - MaxLatency: %v\n", stats.Threshold().MaxLatency())
		fmt.Printf("  Threshold - MinSuccessRate: %.0f%%\n", stats.Threshold().MinSuccessRate()*100)
		if len(stats.BlockedRelayList()) > 0 {
			fmt.Println("  Blocked relay fingerprints:")
			for _, fp := range stats.BlockedRelayList() {
				fmt.Printf("    - %s\n", fp)
			}
		}
	}
}

// customThresholdExample demonstrates how to configure custom thresholds
// for more aggressive slow relay detection.
func customThresholdExample() {
	// Launch Tor daemon
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),
		tornago.WithTorControlAddr(":0"),
		tornago.WithTorStartupTimeout(60*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create launch config: %v", err)
	}

	torProcess, err := tornago.StartTorDaemon(launchCfg)
	if err != nil {
		log.Fatalf("Failed to start Tor: %v", err)
	}
	defer torProcess.Stop()

	// Create client with custom slow relay avoidance settings
	// More aggressive thresholds for low-latency requirements
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientControlAddr(torProcess.ControlAddr()),
		tornago.WithClientRequestTimeout(60*time.Second),
		tornago.WithSlowRelayAvoidance(
			tornago.SlowRelayMaxLatency(3*time.Second),       // Block relays slower than 3s (default: 5s)
			tornago.SlowRelayMinSuccessRate(0.9),             // Require 90% success rate (default: 80%)
			tornago.SlowRelayBlockDuration(1*time.Hour),      // Block for 1 hour (default: 30min)
			tornago.SlowRelayMinSamples(5),                   // Need 5 samples before judging (default: 3)
			tornago.SlowRelayMonitorInterval(15*time.Second), // Check every 15s (default: 30s)
		),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Making requests with strict thresholds...")
	for i := range 5 {
		exitIP := getExitIP(client)
		fmt.Printf("Request %d - Exit IP: %s\n", i+1, exitIP)
	}

	// Check statistics
	stats, ok := client.RelayPerformanceStats()
	if ok {
		fmt.Printf("\nRelay Performance Statistics:\n")
		fmt.Printf("  Tracked relays: %d\n", stats.TrackedRelays())
		fmt.Printf("  Blocked relays: %d\n", stats.BlockedRelays())
	}
}

// checkIPResponse represents the response from check.torproject.org API
type checkIPResponse struct {
	IsTor bool   `json:"IsTor"` //nolint:tagliatelle // JSON tag is from external API
	IP    string `json:"IP"`    //nolint:tagliatelle // JSON tag is from external API
}

// getExitIP retrieves the current Tor exit IP address
func getExitIP(client *tornago.Client) string {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://check.torproject.org/api/ip",
		http.NoBody,
	)
	if err != nil {
		return errorResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		return errorResponse
	}
	defer resp.Body.Close()

	var result checkIPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errorResponse
	}

	if result.IP == "" {
		return "unknown"
	}

	return result.IP
}
