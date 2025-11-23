// Package main provides an example HTTP server accessible via Tor hidden service.
//
// This example demonstrates creating a Hidden Service (.onion address):
//  1. Launch a Tor daemon
//  2. Start a local HTTP server (listening on 127.0.0.1, NOT publicly accessible)
//  3. Create a Hidden Service that maps a .onion address to the local server
//  4. The service becomes accessible via Tor without exposing your IP
//
// Architecture:
//
//	┌─────────────────┐                 ┌──────────────────┐
//	│  Tor Network    │                 │  Your Machine    │
//	│                 │                 │                  │
//	│  .onion:80 ────────────────────> │  localhost:8080  │
//	│  (public)       │   Port Mapping  │  (private)       │
//	└─────────────────┘                 └──────────────────┘
//
// Key Concepts:
//
// Port Mapping (80 -> 8080):
//   - Virtual port 80: What clients see when accessing your .onion address
//   - Target port 8080: Your actual local HTTP server port
//   - Tor handles the port forwarding automatically
//
// Security Model:
//   - Your HTTP server is ONLY accessible via Tor (bound to 127.0.0.1)
//   - Your real IP address is never exposed to clients
//   - Clients are also anonymous to you
//   - No need for public IP, DNS, or firewall configuration
//
// Use Cases:
//   - Anonymous blogging/publishing
//   - Whistleblower drop boxes
//   - Privacy-focused services
//   - Development/testing without exposing localhost
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nao1215/tornago"
)

func main() {
	// Step 1: Launch Tor daemon
	// NOTE: Same as client examples. The Tor instance handles both
	// outbound SOCKS connections AND ControlPort commands for Hidden Services.
	fmt.Println("Starting Tor daemon...")
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),   // SOCKS port (not used in this example)
		tornago.WithTorControlAddr(":0"), // ControlPort (REQUIRED for Hidden Services)
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

	// Step 2: Start local HTTP server
	// WHY 127.0.0.1: Binds to localhost only, NOT publicly accessible.
	// Your server is only reachable via the Tor Hidden Service, not directly.
	// Security: Never bind to 0.0.0.0 for Hidden Services unless you want
	// the service accessible both via Tor AND directly via your IP.
	localAddr := "127.0.0.1:8080"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
    <title>Tornago Hidden Service</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #7d4698;
        }
        .info {
            background-color: #f0e6f6;
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
        }
        code {
            background-color: #e0e0e0;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: monospace;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧅 Welcome to Tornago Hidden Service!</h1>
        <p>This is a simple web page hosted as a Tor Hidden Service (.onion) using the <strong>tornago</strong> library.</p>

        <div class="info">
            <h3>Connection Info:</h3>
            <p><strong>Your IP:</strong> <code>` + r.RemoteAddr + `</code></p>
            <p><strong>Request Path:</strong> <code>` + r.URL.Path + `</code></p>
            <p><strong>User Agent:</strong> <code>` + r.UserAgent() + `</code></p>
        </div>

        <h3>About Tornago:</h3>
        <p>Tornago is a lightweight Go wrapper around the Tor command-line tool, providing:</p>
        <ul>
            <li>Tor Daemon Management</li>
            <li>Tor Client (SOCKS5 proxy)</li>
            <li>Tor Server (Hidden Services)</li>
        </ul>

        <p style="margin-top: 30px; text-align: center; color: #666;">
            Powered by <strong>tornago</strong> 🚀
        </p>
    </div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})

	server := &http.Server{
		Addr:              localAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // Prevents Slowloris attacks
	}

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", localAddr)
	if err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	// WHY goroutine: Start HTTP server in background so we can continue
	// to create the Hidden Service mapping. Server must be running before
	// clients can access the .onion address.
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	fmt.Printf("\nLocal HTTP server started on http://%s\n", localAddr)

	// Step 3: Get control authentication and create ControlClient directly
	// WHY authentication needed: Tor's ControlPort requires authentication
	// to prevent unauthorized access. ControlAuthFromTor() automatically
	// detects the auth method (cookie or password) configured for this Tor instance.
	fmt.Println("\nObtaining Tor control authentication...")
	auth, _, err := tornago.ControlAuthFromTor(torProcess.ControlAddr(), 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get control auth: %v", err)
	}

	// Step 4: Create ControlClient directly (instead of via tornago.Client)
	// NOTE: ControlClient is the low-level API for ControlPort commands.
	// For Hidden Services, we use it directly. For SOCKS proxy, we use tornago.Client.
	controlClient, err := tornago.NewControlClient(
		torProcess.ControlAddr(),
		auth,
		30*time.Second, // Command timeout
	)
	if err != nil {
		log.Fatalf("Failed to create control client: %v", err)
	}
	defer controlClient.Close()

	// WHY Authenticate: Even with valid credentials, we must explicitly
	// authenticate before sending any ControlPort commands.
	if err := controlClient.Authenticate(); err != nil {
		log.Fatalf("Failed to authenticate with Tor: %v", err)
	}

	// Step 5: Create Hidden Service
	// WHY port mapping (80 -> 8080):
	//   - Virtual port 80: Standard HTTP port, what clients use in browser
	//   - Target port 8080: Our actual local server port
	//   - Example: http://abc123.onion:80 -> localhost:8080
	// NOTE: This is ephemeral (temporary). The .onion address changes on restart.
	// See examples/persistent_onion for keeping the same address.
	hsCfg, err := tornago.NewHiddenServiceConfig(
		tornago.WithHiddenServicePort(80, 8080), // Map onion port 80 to local port 8080
	)
	if err != nil {
		log.Fatalf("Failed to create hidden service config: %v", err)
	}

	fmt.Println("\nCreating Hidden Service...")
	// WHY CreateHiddenService: Sends ADD_ONION command to Tor's ControlPort.
	// Tor generates a new ED25519 keypair and returns the .onion address.
	// The service is immediately accessible once this succeeds.
	hs, err := controlClient.CreateHiddenService(context.Background(), hsCfg)
	if err != nil {
		log.Fatalf("Failed to create hidden service: %v", err)
	}
	// WHY defer Remove(): Cleanup the Hidden Service on exit.
	// For ephemeral services, this is good practice. For persistent services,
	// you might want to keep it registered.
	defer func() {
		if err := hs.Remove(context.Background()); err != nil {
			log.Printf("Failed to delete hidden service: %v", err)
		}
	}()

	fmt.Printf("\n✅ Hidden Service created successfully!\n")
	fmt.Printf("   Onion Address: http://%s\n", hs.OnionAddress())
	fmt.Printf("   Local Address: http://%s\n", localAddr)
	fmt.Println("\nYou can access this hidden service through Tor using the onion address above.")
	fmt.Println("Press Ctrl+C to stop the server...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}
