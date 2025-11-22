// Package main demonstrates advanced circuit management features including
// automatic circuit rotation, manual rotation, and circuit prewarming.
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

func main() {
	// Example 1: Manual Circuit Rotation
	fmt.Println("=== Example 1: Manual Circuit Rotation ===")
	manualRotationExample()

	// Example 2: Automatic Circuit Rotation
	fmt.Println("\n=== Example 2: Automatic Circuit Rotation ===")
	autoRotationExample()

	// Example 3: Circuit Prewarming
	fmt.Println("\n=== Example 3: Circuit Prewarming ===")
	prewarmingExample()

	// Example 4: Connection Reuse Metrics
	fmt.Println("\n=== Example 4: Connection Reuse Metrics ===")
	connectionReuseExample()
}

func manualRotationExample() {
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

	// Create client with ControlPort access
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientControlAddr(torProcess.ControlAddr()),
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

	// Create circuit manager
	manager := tornago.NewCircuitManager(client.Control())

	// Make first request to see initial exit IP
	exitIP1 := getExitIP(client)
	fmt.Printf("Initial exit IP: %s\n", exitIP1)

	// Rotate circuit manually
	fmt.Println("Rotating circuit manually...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := manager.RotateNow(ctx); err != nil {
		log.Fatalf("Circuit rotation failed: %v", err)
	}

	// Wait for new circuit to be established
	fmt.Println("Waiting for new circuit to establish (10 seconds)...")
	time.Sleep(10 * time.Second)

	// Make request with new circuit
	exitIP2 := getExitIP(client)
	fmt.Printf("After rotation exit IP: %s\n", exitIP2)

	if exitIP1 != exitIP2 {
		fmt.Println("✓ Circuit successfully rotated (IP changed)")
	} else {
		fmt.Println("⚠ Circuit rotated but IP remained the same (may happen randomly)")
	}
}

func autoRotationExample() {
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

	// Create client
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientControlAddr(torProcess.ControlAddr()),
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

	// Create circuit manager with logger
	manager := tornago.NewCircuitManager(client.Control())

	// Start automatic rotation every 10 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	rotationInterval := 10 * time.Second
	fmt.Printf("Starting automatic rotation every %v\n", rotationInterval)

	if err := manager.StartAutoRotation(ctx, rotationInterval); err != nil {
		log.Fatalf("Failed to start auto-rotation: %v", err)
	}
	defer manager.Stop()

	// Check stats
	stats := manager.Stats()
	fmt.Printf("Auto-rotation enabled: %v\n", stats.AutoRotationEnabled)
	fmt.Printf("Rotation interval: %v\n", stats.RotationInterval)

	// Make requests periodically to observe IP changes
	for i := range 3 {
		exitIP := getExitIP(client)
		fmt.Printf("Request %d - Exit IP: %s\n", i+1, exitIP)
		time.Sleep(12 * time.Second) // Wait for rotation to occur
	}

	fmt.Println("Auto-rotation test completed")
}

func prewarmingExample() {
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

	// Create client
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientControlAddr(torProcess.ControlAddr()),
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

	// Create circuit manager
	manager := tornago.NewCircuitManager(client.Control())

	// Prewarm circuits before making batch requests
	fmt.Println("Prewarming circuits before batch requests...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := manager.PrewarmCircuits(ctx); err != nil {
		log.Fatalf("Circuit prewarming failed: %v", err)
	}

	// Wait for circuits to establish
	fmt.Println("Waiting for prewarmed circuits to establish (10 seconds)...")
	time.Sleep(10 * time.Second)

	// Now make batch requests (will use prewarmed circuits)
	fmt.Println("Making batch requests...")
	for i := range 3 {
		exitIP := getExitIP(client)
		fmt.Printf("Request %d - Exit IP: %s\n", i+1, exitIP)
	}

	fmt.Println("✓ Batch requests completed using prewarmed circuits")
}

func connectionReuseExample() {
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

	// Create client with metrics
	metrics := tornago.NewMetricsCollector()

	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientRequestTimeout(60*time.Second),
		tornago.WithClientMetrics(metrics),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Make multiple requests to same host to observe connection reuse
	fmt.Println("Making 10 requests to https://check.torproject.org...")
	for i := range 10 {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"https://check.torproject.org/api/ip",
			http.NoBody,
		)
		if err != nil {
			log.Printf("Request %d failed to create: %v", i+1, err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
			continue
		}
		_ = resp.Body.Close()
	}

	// Display connection reuse metrics
	fmt.Println("\n=== Connection Reuse Metrics ===")
	fmt.Printf("Total requests: %d\n", metrics.RequestCount())
	fmt.Printf("Successful requests: %d\n", metrics.SuccessCount())
	fmt.Printf("Failed requests: %d\n", metrics.ErrorCount())
	fmt.Printf("Total dial operations: %d\n", metrics.DialCount())
	fmt.Printf("Connections reused: %d\n", metrics.ConnectionReuseCount())
	fmt.Printf("Connection reuse rate: %.2f%%\n", metrics.ConnectionReuseRate()*100)

	fmt.Println("\nConnection Pooling Efficiency:")
	reuseRate := metrics.ConnectionReuseRate()
	if reuseRate > 0.7 {
		fmt.Println("✓ Excellent - High connection reuse (> 70%)")
	} else if reuseRate > 0.4 {
		fmt.Println("⚠ Good - Moderate connection reuse (40-70%)")
	} else {
		fmt.Println("⚠ Low connection reuse (< 40%) - Consider increasing MaxIdleConnsPerHost")
	}

	fmt.Printf("\nAverage latency: %v\n", metrics.AverageLatency())
	fmt.Printf("Min latency: %v\n", metrics.MinLatency())
	fmt.Printf("Max latency: %v\n", metrics.MaxLatency())
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
		return "error creating request"
	}

	resp, err := client.Do(req)
	if err != nil {
		return "error making request"
	}
	defer resp.Body.Close()

	var result checkIPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "error parsing response"
	}

	if result.IP == "" {
		return "unknown"
	}

	return result.IP
}
