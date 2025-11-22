// Package main demonstrates connecting to an existing Tor daemon.
// This is the recommended approach for production environments where
// a system Tor daemon is already running.
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
	// Connect to system Tor daemon (default ports on Debian/Ubuntu)
	// Install: sudo apt install tor
	// The default configuration typically has:
	//   SocksPort 127.0.0.1:9050
	//   ControlPort 127.0.0.1:9051
	fmt.Println("Connecting to existing Tor daemon...")

	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr("127.0.0.1:9050"),
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

	// Make a request to verify Tor connection
	fmt.Println("Verifying Tor connection...")
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"https://check.torproject.org/api/ip",
		http.NoBody,
	)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("Response from check.torproject.org:\n%s\n", string(body))
}
