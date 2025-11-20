// Package tornago provides a Tor client/server helper library that can launch
// tor for development and connect to existing Tor instances in production.
//
// # What is Tor?
//
// Tor (The Onion Router) is a network of relays that anonymizes internet traffic
// by routing connections through multiple encrypted hops. Key concepts:
//
//   - SocksPort: The SOCKS5 proxy port that applications use to route traffic through Tor.
//     Think of it as the "entrance" to the Tor network for your application's outbound connections.
//
//   - ControlPort: A text-based management interface for controlling a running Tor instance.
//     Used for operations like rotating circuits (NewIdentity), creating hidden services,
//     and querying Tor's internal state.
//
//   - Hidden Service (Onion Service): A service accessible only through the Tor network,
//     identified by a .onion address. This allows you to host servers that are both
//     anonymous and accessible without requiring a public IP address or DNS registration.
//
//   - Circuit: The path your traffic takes through multiple Tor relays. Each circuit
//     typically consists of 3 relays (guard, middle, exit) for outbound connections.
//
// # Quick Start
//
// For the simplest use case (making HTTP requests through Tor), you need:
//
//  1. A running Tor instance (installed via package manager or launched by tornago)
//  2. A Client configured with the Tor SocksPort address
//  3. Use the Client's HTTP() method to get an *http.Client that routes through Tor
//
// See Example_quickStart for a complete minimal example.
//
// # Main Use Cases
//
// **Making HTTP requests through Tor** (most common):
//   - Create a Client pointing to a Tor SocksPort
//   - Use client.HTTP() to get an *http.Client that routes through Tor
//   - All HTTP requests automatically go through Tor's anonymizing network
//
// **Launching Tor programmatically** (development/testing):
//   - Use StartTorDaemon() to launch a tor process managed by your application
//   - tornago handles port allocation, startup synchronization, and cleanup
//   - Useful when you don't want to require users to install/configure Tor separately
//
// **Creating Hidden Services** (hosting anonymous servers):
//   - Use ControlClient.CreateHiddenService() to create a .onion address
//   - Map your local server port to a virtual onion port
//   - Your service becomes accessible via Tor without exposing your IP address
//
// **Rotating Tor circuits** (getting a new IP address):
//   - Use ControlClient.NewIdentity() to signal Tor to build new circuits
//   - Subsequent requests will use different exit nodes (different public IPs)
//   - Useful for rate-limiting evasion or additional anonymity
//
// # Architecture Overview
//
// tornago provides several components that work together:
//
//   - Client: High-level HTTP/TCP client with automatic Tor routing and retry logic
//   - ControlClient: Low-level interface to Tor's ControlPort for management commands
//   - TorProcess: Represents a tor daemon launched by StartTorDaemon()
//   - Server: Simple wrapper for existing Tor instance addresses
//   - HiddenService: Represents a created .onion service
//
// All configurations use functional options pattern for flexibility and immutability.
//
// # Authentication
//
// Tor's ControlPort requires authentication. tornago supports:
//
//   - Cookie authentication (default): Tor writes a random cookie file, tornago reads it
//   - Password authentication: You configure a hashed password in Tor and provide it to tornago
//
// When using StartTorDaemon(), cookie authentication is configured automatically.
// When connecting to an existing Tor instance, you must provide appropriate credentials.
//
// # Error Handling
//
// All tornago errors are wrapped in TornagoError with a Kind field for programmatic handling.
// Use errors.Is() to check error kinds:
//
//	if errors.Is(err, &tornago.TornagoError{Kind: tornago.ErrSocksDialFailed}) {
//	    // Handle connection failure
//	}
//
// Common error kinds:
//   - ErrTorBinaryNotFound: tor executable not in PATH (install via package manager)
//   - ErrSocksDialFailed: Cannot connect to Tor SocksPort (is Tor running?)
//   - ErrControlRequestFail: ControlPort command failed (check authentication)
//   - ErrTimeout: Operation exceeded deadline (increase timeout or check network)
package tornago
