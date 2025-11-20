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

// Example_quickStart demonstrates the simplest way to make HTTP requests through Tor.
// This example assumes you have Tor running locally on the default port (9050).
// To install Tor: apt-get install tor (Ubuntu), brew install tor (macOS), or choco install tor (Windows).
func Example_quickStart() {
	// Create a client that uses your local Tor instance
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr("127.0.0.1:9050"),
	)
	if err != nil {
		log.Fatalf("failed to create config: %v", err)
	}

	client, err := tornago.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Get the HTTP client and use it like any http.Client
	// All requests automatically go through Tor
	httpClient := client.HTTP()
	_ = httpClient // Use for http.Get, http.Post, etc.

	fmt.Println("HTTP client ready to make requests through Tor")
	// Output: HTTP client ready to make requests through Tor
}

// Example_launchAndUse demonstrates launching a Tor daemon and using it for HTTP requests.
// This is useful when you want your application to manage its own Tor instance.
func Example_launchAndUse() {
	// Launch a Tor daemon with automatic port selection
	launchCfg, err := tornago.NewTorLaunchConfig(
		tornago.WithTorSocksAddr(":0"),   // Let Tor pick a free port
		tornago.WithTorControlAddr(":0"), // Let Tor pick a free port
		tornago.WithTorStartupTimeout(time.Minute),
	)
	if err != nil {
		log.Fatalf("failed to create launch config: %v", err)
	}

	// StartTorDaemon blocks until Tor is ready to accept connections
	// (Note: This example doesn't actually start Tor to keep tests fast)
	// torProc, err := tornago.StartTorDaemon(launchCfg)
	// if err != nil {
	//     log.Fatalf("failed to start tor: %v", err)
	// }
	// defer torProc.Stop()

	// Create a client using the launched Tor instance
	// clientCfg, err := tornago.NewClientConfig(
	//     tornago.WithClientSocksAddr(torProc.SocksAddr()),
	// )
	// client, err := tornago.NewClient(clientCfg)

	fmt.Printf("Configured to launch Tor with timeout: %v\n", launchCfg.StartupTimeout())
	// Output: Configured to launch Tor with timeout: 1m0s
}

// Example_connectToExistingTor demonstrates connecting to an already-running Tor instance.
// This is the recommended approach for production environments.
func Example_connectToExistingTor() {
	// Configure client to connect to a running Tor instance
	// (Note: This example shows configuration only; actual connection requires Tor to be running)
	clientCfg, err := tornago.NewClientConfig(
		tornago.WithClientSocksAddr("127.0.0.1:9050"),
		// Optionally configure ControlPort for circuit rotation and hidden services
		// tornago.WithClientControlAddr("127.0.0.1:9051"),
		// tornago.WithClientControlCookie("/var/lib/tor/control_auth_cookie"),
	)
	if err != nil {
		log.Fatalf("failed to create config: %v", err)
	}

	// In production, you would create and use the client like this:
	// client, err := tornago.NewClient(clientCfg)
	// if err != nil {
	//     log.Fatalf("failed to create client: %v", err)
	// }
	// defer client.Close()

	fmt.Printf("Configured to connect to Tor at %s\n", clientCfg.SocksAddr())
	// Output: Configured to connect to Tor at 127.0.0.1:9050
}

// Example_rotateCircuit demonstrates how to request a new Tor identity (new exit node/IP).
func Example_rotateCircuit() {
	// This example shows the concept; actual execution requires a running Tor instance
	fmt.Println("To rotate circuits:")
	fmt.Println("1. Create a Client with ControlAddr configured")
	fmt.Println("2. Call client.Control().NewIdentity(ctx)")
	fmt.Println("3. Subsequent requests use new circuits with different exit IPs")
	// Output: To rotate circuits:
	// 1. Create a Client with ControlAddr configured
	// 2. Call client.Control().NewIdentity(ctx)
	// 3. Subsequent requests use new circuits with different exit IPs
}

// Example_errorHandling demonstrates how to handle tornago-specific errors.
func Example_errorHandling() {
	fmt.Println("tornago uses TornagoError with Kind field for error classification")
	fmt.Println("Use errors.Is() to check specific error kinds")
	fmt.Println("Common kinds: ErrTorBinaryNotFound, ErrSocksDialFailed, ErrTimeout")
	// Output: tornago uses TornagoError with Kind field for error classification
	// Use errors.Is() to check specific error kinds
	// Common kinds: ErrTorBinaryNotFound, ErrSocksDialFailed, ErrTimeout
}
