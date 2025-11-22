// Package main demonstrates metrics collection and rate limiting.
// Useful for monitoring and controlling request rates in production.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Launch Tor daemon
	fmt.Println("Starting Tor daemon...")
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
		log.Fatalf("Failed to start Tor daemon: %v", err)
	}
	defer torProcess.Stop()

	fmt.Printf("Tor started (SOCKS: %s)\n\n", torProcess.SocksAddr())

	// Create metrics collector
	metrics := tornago.NewMetricsCollector()

	// Create rate limiter: 2 requests per second, burst of 5
	rateLimiter := tornago.NewRateLimiter(2.0, 5)

	// Create client with metrics and rate limiting
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientRequestTimeout(30*time.Second),
		tornago.WithClientMetrics(metrics),
		tornago.WithClientRateLimiter(rateLimiter),
	)
	if err != nil {
		log.Fatalf("Failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Make multiple requests to demonstrate rate limiting
	fmt.Println("Making 10 requests (rate limited to 2 req/s)...")
	urls := []string{
		"https://example.com",
		"https://example.org",
		"https://example.net",
		"https://example.com",
		"https://example.org",
		"https://example.net",
		"https://example.com",
		"https://example.org",
		"https://example.net",
		"https://example.com",
	}

	start := time.Now()
	for i, url := range urls {
		reqStart := time.Now()
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			url,
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
		if resp.Body != nil {
			_ = resp.Body.Close()
		}

		elapsed := time.Since(reqStart)
		fmt.Printf("Request %2d: %s (took %v)\n", i+1, resp.Status, elapsed.Round(time.Millisecond))
	}

	totalElapsed := time.Since(start)
	fmt.Printf("\nTotal time: %v\n\n", totalElapsed.Round(time.Millisecond))

	// Display metrics
	fmt.Println("Metrics Summary:")
	fmt.Printf("  Total requests: %d\n", metrics.RequestCount())
	fmt.Printf("  Successful requests: %d\n", metrics.SuccessCount())
	fmt.Printf("  Failed requests: %d\n", metrics.ErrorCount())
	if metrics.RequestCount() > 0 {
		errorRate := float64(metrics.ErrorCount()) / float64(metrics.RequestCount()) * 100
		fmt.Printf("  Error rate: %.2f%%\n", errorRate)
	}
	fmt.Printf("  Average latency: %v\n", metrics.AverageLatency().Round(time.Millisecond))
	fmt.Printf("  Total latency: %v\n", metrics.TotalLatency().Round(time.Millisecond))
}
