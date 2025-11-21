// Package main provides an example HTTP server accessible via Tor hidden service.
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
	fmt.Println("Starting Tor daemon...")
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),   // Use random available port
		tornago.WithTorControlAddr(":0"), // Use random available port
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

	fmt.Printf("\nLocal HTTP server started on http://%s\n", localAddr)

	// Step 3: Get control authentication and create ControlClient directly
	fmt.Println("\nObtaining Tor control authentication...")
	auth, _, err := tornago.ControlAuthFromTor(torProcess.ControlAddr(), 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get control auth: %v", err)
	}

	// Step 4: Create ControlClient directly (instead of via tornago.Client)
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
		log.Fatalf("Failed to authenticate with Tor: %v", err)
	}

	// Step 5: Create Hidden Service
	hsCfg, err := tornago.NewHiddenServiceConfig(
		tornago.WithHiddenServicePort(80, 8080), // Map onion port 80 to local port 8080
	)
	if err != nil {
		log.Fatalf("Failed to create hidden service config: %v", err)
	}

	fmt.Println("\nCreating Hidden Service...")
	hs, err := controlClient.CreateHiddenService(context.Background(), hsCfg)
	if err != nil {
		log.Fatalf("Failed to create hidden service: %v", err)
	}
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
