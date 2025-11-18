package tornago_test

import (
	"fmt"
	"log"
	"time"

	"github.com/nao1215/tornago"
)

// Example_client demonstrates how to create a Tor client configuration
// for making HTTP requests through the Tor network.
func Example_client() {
	// Create a client configuration with default settings
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr("127.0.0.1:9050"), // Use local Tor SOCKS proxy
	)
	if err != nil {
		log.Fatalf("failed to create client config: %v", err)
	}

	// Create a new Tor client
	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Printf("Client configured with SOCKS: %s\n", clientCfg.SocksAddr())
	// Output: Client configured with SOCKS: 127.0.0.1:9050
}

// Example_hiddenService demonstrates how to configure a Tor hidden service
// (onion service) that can be accessed via the Tor network.
func Example_hiddenService() {
	// Configure a hidden service that maps port 80 to local port 8080
	hsCfg, err := tornago.NewHiddenServiceConfig(
		tornago.WithHiddenServicePort(80, 8080),
	)
	if err != nil {
		log.Fatalf("failed to create hidden service config: %v", err)
	}

	fmt.Printf("Hidden service configured: virtual port %d -> local port %d\n", 80, 8080)
	fmt.Printf("Key type: %s\n", hsCfg.KeyType())
	// Output: Hidden service configured: virtual port 80 -> local port 8080
	// Key type: ED25519-V3
}

// Example_server demonstrates how to start a Tor server that can host
// hidden services and handle incoming connections.
func Example_server() {
	// Create server configuration
	serverCfg, err := tornago.NewServerConfig(
		tornago.WithServerSocksAddr("127.0.0.1:9050"),
		tornago.WithServerControlAddr("127.0.0.1:9051"),
	)
	if err != nil {
		log.Fatalf("failed to create server config: %v", err)
	}

	// Create a server instance
	server, err := tornago.NewServer(serverCfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	fmt.Printf("Tor server configured with SOCKS: %s, Control: %s\n",
		server.SocksAddr(), server.ControlAddr())
	// Output: Tor server configured with SOCKS: 127.0.0.1:9050, Control: 127.0.0.1:9051
}

// Example_controlClient demonstrates how to configure a control client
// for interacting with a running Tor instance.
func Example_controlClient() {
	// Create a control client configuration with password authentication
	auth := tornago.ControlAuthFromPassword("my-password")

	fmt.Printf("Control auth configured with password\n")
	_ = auth // Use auth variable
	// Output: Control auth configured with password
}

// Example_newIdentity demonstrates the concept of requesting a new Tor identity.
// In practice, you would call client.NewIdentity(ctx) on an authenticated control client
// to rotate circuits and get a new IP address.
func Example_newIdentity() {
	fmt.Println("To request a new identity, use client.NewIdentity(ctx)")
	// Output: To request a new identity, use client.NewIdentity(ctx)
}

// Example_clientWithRetry demonstrates how to configure retry behavior
// for HTTP requests through Tor.
func Example_clientWithRetry() {
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr("127.0.0.1:9050"),
		tornago.WithRetryAttempts(3),
		tornago.WithRetryDelay(2*time.Second),
	)
	if err != nil {
		log.Fatalf("failed to create client config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Printf("Client configured with 3 retry attempts and 2s delay\n")
	// Output: Client configured with 3 retry attempts and 2s delay
}

// Example_hiddenServiceWithAuth demonstrates how to create a hidden service
// with client authorization for restricted access.
func Example_hiddenServiceWithAuth() {
	// Configure hidden service with client authorization
	auth := tornago.NewHiddenServiceAuth("alice", "descriptor:x25519:ABCDEF1234567890")

	_, err := tornago.NewHiddenServiceConfig(
		tornago.WithHiddenServicePort(80, 8080),
		tornago.WithHiddenServiceClientAuth(auth),
	)
	if err != nil {
		log.Fatalf("failed to create config: %v", err)
	}

	fmt.Printf("Configured hidden service with auth for: %s\n", auth.ClientName())
	// Output: Configured hidden service with auth for: alice
}

// Example_startTorDaemon demonstrates how to configure a Tor daemon
// launch configuration with custom settings.
func Example_startTorDaemon() {
	// Create launch configuration with custom settings
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr("127.0.0.1:9050"),
		tornago.WithTorControlAddr("127.0.0.1:9051"),
		tornago.WithTorStartupTimeout(2*time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to create launch config: %v", err)
	}

	fmt.Printf("Tor daemon configured with SOCKS: %s, Control: %s\n",
		launchCfg.SocksAddr(), launchCfg.ControlAddr())
	// Output: Tor daemon configured with SOCKS: 127.0.0.1:9050, Control: 127.0.0.1:9051
}
