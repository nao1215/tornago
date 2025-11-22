// Package main demonstrates observability features: logging, metrics, and health checks.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Example 1: Structured logging with slog
	fmt.Println("=== Example 1: Structured Logging ===")

	// Create a slog logger with JSON output
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	tornagoLogger := tornago.NewSlogAdapter(slogLogger)

	// Launch Tor with logging enabled
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

	fmt.Printf("Tor started (SOCKS: %s, Control: %s)\n\n",
		torProcess.SocksAddr(), torProcess.ControlAddr())

	// Create client with metrics and logging
	metrics := tornago.NewMetricsCollector()
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr(torProcess.SocksAddr()),
		tornago.WithClientDialTimeout(30*time.Second),
		tornago.WithClientRequestTimeout(60*time.Second),
		tornago.WithClientLogger(tornagoLogger),
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

	// Example 2: Health checks
	fmt.Println("\n=== Example 2: Health Checks ===")

	health := client.Check(context.Background())
	fmt.Printf("Client health: %s\n", health)
	fmt.Printf("  Status: %s\n", health.Status())
	fmt.Printf("  Message: %s\n", health.Message())
	fmt.Printf("  Latency: %v\n", health.Latency())
	fmt.Printf("  Timestamp: %s\n", health.Timestamp().Format(time.RFC3339))

	// Check using query methods
	if health.IsHealthy() {
		fmt.Println("  ✓ Client is healthy")
	} else if health.IsDegraded() {
		fmt.Println("  ⚠ Client is degraded")
	} else if health.IsUnhealthy() {
		fmt.Println("  ✗ Client is unhealthy")
	}

	// Example 3: TorProcess health check
	fmt.Println("\n=== Example 3: Tor Daemon Health Check ===")

	daemonHealth := tornago.CheckTorDaemon(context.Background(), torProcess)
	fmt.Printf("Tor daemon health: %s\n", daemonHealth)

	if daemonHealth.IsHealthy() {
		fmt.Println("  ✓ Daemon is healthy")
	} else if daemonHealth.IsDegraded() {
		fmt.Printf("  ⚠ Daemon is degraded: %s\n", daemonHealth.Message())
	} else {
		fmt.Printf("  ✗ Daemon is unhealthy: %s\n", daemonHealth.Message())
	}

	// Example 4: Metrics collection
	fmt.Println("\n=== Example 4: Metrics Collection ===")

	// Make several requests
	urls := []string{
		"https://check.torproject.org",
		"https://www.torproject.org",
		"https://example.com",
	}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			url,
			http.NoBody,
		)
		if err != nil {
			log.Printf("Failed to create request: %v", err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Request to %s failed: %v", url, err)
			continue
		}
		_ = resp.Body.Close()
		fmt.Printf("  %s: HTTP %d\n", url, resp.StatusCode)
	}

	// Display metrics
	fmt.Println("\nMetrics Summary:")
	fmt.Printf("  Total requests: %d\n", metrics.RequestCount())
	fmt.Printf("  Successful: %d\n", metrics.SuccessCount())
	fmt.Printf("  Failed: %d\n", metrics.ErrorCount())
	fmt.Printf("  Average latency: %v\n", metrics.AverageLatency())
	fmt.Printf("  Min latency: %v\n", metrics.MinLatency())
	fmt.Printf("  Max latency: %v\n", metrics.MaxLatency())
	fmt.Printf("  Total latency: %v\n", metrics.TotalLatency())

	errorsByKind := metrics.ErrorsByKind()
	if len(errorsByKind) > 0 {
		fmt.Println("\n  Errors by kind:")
		for kind, count := range errorsByKind {
			fmt.Printf("    %s: %d\n", kind, count)
		}
	}

	// Example 5: Metrics reset
	fmt.Println("\n=== Example 5: Metrics Reset ===")
	fmt.Printf("Before reset - Request count: %d\n", metrics.RequestCount())
	metrics.Reset()
	fmt.Printf("After reset - Request count: %d\n", metrics.RequestCount())

	// Example 6: Periodic health monitoring
	fmt.Println("\n=== Example 6: Periodic Health Monitoring ===")
	fmt.Println("Running health checks every 2 seconds for 6 seconds...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	timeout := time.After(6 * time.Second)
	checkCount := 0

	for {
		select {
		case <-ticker.C:
			checkCount++
			h := client.Check(context.Background())
			fmt.Printf("  Check %d: %s (latency: %v)\n",
				checkCount, h.Status(), h.Latency())

		case <-timeout:
			fmt.Println("\nHealth monitoring complete")
			fmt.Println("\n=== All Examples Complete ===")
			return
		}
	}
}
