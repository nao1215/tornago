// Package main demonstrates creating a Hidden Service with a persistent private key.
// This allows the .onion address to remain the same across restarts.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Key file path
	keyFile := filepath.Join(os.TempDir(), "tornago-example-hs.key")

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

	fmt.Printf("Tor started (Control: %s)\n", torProcess.ControlAddr())

	// Get control authentication
	auth, _, err := tornago.ControlAuthFromTor(torProcess.ControlAddr(), 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get control auth: %v", err)
	}

	// Create ControlClient
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

	// Check if key file exists
	var hsCfg tornago.HiddenServiceConfig
	if _, err := os.Stat(keyFile); err == nil {
		// Load existing key
		fmt.Printf("Loading existing private key from: %s\n", keyFile)
		hsCfg, err = tornago.NewHiddenServiceConfig(
			tornago.WithHiddenServicePrivateKeyFile(keyFile),
			tornago.WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			log.Fatalf("Failed to create config with existing key: %v", err)
		}
	} else {
		// Create new key
		fmt.Println("Creating new private key (first run)")
		hsCfg, err = tornago.NewHiddenServiceConfig(
			tornago.WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			log.Fatalf("Failed to create config: %v", err)
		}
	}

	// Start local HTTP server
	localAddr := "127.0.0.1:8080"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from persistent Hidden Service!\nTime: %s\n", time.Now().Format(time.RFC3339))
	})

	server := &http.Server{
		Addr:              localAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", localAddr)
	if err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	fmt.Printf("Local HTTP server started on http://%s\n", localAddr)

	// Create Hidden Service
	fmt.Println("Creating Hidden Service...")
	hs, err := controlClient.CreateHiddenService(context.Background(), hsCfg)
	if err != nil {
		log.Fatalf("Failed to create hidden service: %v", err)
	}
	defer hs.Remove(context.Background())

	// Save private key for future use
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		fmt.Printf("Saving private key to: %s\n", keyFile)
		if err := hs.SavePrivateKey(keyFile); err != nil {
			log.Fatalf("Failed to save private key: %v", err)
		}
		fmt.Println("Key saved. Restart this program to reuse the same .onion address")
	}

	fmt.Printf("\nHidden Service ready!\n")
	fmt.Printf("  Onion Address: http://%s\n", hs.OnionAddress())
	fmt.Printf("  Local Address: http://%s\n", localAddr)
	fmt.Printf("  Private Key: %s\n", keyFile)
	fmt.Println("\nPress Ctrl+C to stop...")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}
