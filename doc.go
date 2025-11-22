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
//
// # Configuration
//
// **Timeout Recommendations**
//
// Tor adds significant latency due to multi-hop routing:
//   - Dial timeout: 20-30 seconds for production, 30-60 seconds for .onion sites
//   - Request timeout: 60-120 seconds for typical requests, 120-300 seconds for large downloads
//   - Startup timeout: 60-120 seconds for first launch, 30-60 seconds with cached state
//
// **Development Environment**
//
// Launch ephemeral Tor daemon for testing:
//
//	launchCfg, _ := tornago.NewTorLaunchConfig(
//	    tornago.WithTorSocksAddr(":0"),  // Random port
//	    tornago.WithTorControlAddr(":0"),
//	    tornago.WithTorStartupTimeout(60*time.Second),
//	)
//	torProcess, _ := tornago.StartTorDaemon(launchCfg)
//	defer torProcess.Stop()
//
//	clientCfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr(torProcess.SocksAddr()),
//	    tornago.WithClientRequestTimeout(30*time.Second),
//	)
//
// **Production Environment**
//
// Connect to system Tor daemon:
//
//	clientCfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	    tornago.WithClientDialTimeout(30*time.Second),
//	    tornago.WithClientRequestTimeout(120*time.Second),
//	)
//
// System Tor configuration (/etc/tor/torrc):
//
//	SocksPort 127.0.0.1:9050
//	ControlPort 127.0.0.1:9051
//	CookieAuthentication 1
//
// **With Metrics and Rate Limiting**
//
//	metrics := tornago.NewMetricsCollector()
//	rateLimiter := tornago.NewRateLimiter(5.0, 10)  // 5 req/s, burst 10
//
//	clientCfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	    tornago.WithClientMetrics(metrics),
//	    tornago.WithClientRateLimiter(rateLimiter),
//	)
//
//	// Check metrics
//	fmt.Printf("Requests: %d, Success: %d, Avg latency: %v\n",
//	    metrics.RequestCount(), metrics.SuccessCount(), metrics.AverageLatency())
//
// # Troubleshooting
//
// **Tor binary not found**
//
//	Error: tor_binary_not_found: tor executable not found in PATH
//	Solution: Install Tor via package manager
//	  Ubuntu/Debian: sudo apt install tor
//	  macOS: brew install tor
//	  Verify: tor --version
//
// **Cannot connect to Tor daemon**
//
//	Error: control_request_failed: failed to dial ControlPort
//	Solution: Verify Tor is running
//	  ps aux | grep tor
//	  sudo netstat -tlnp | grep tor
//	  Check /etc/tor/torrc for correct SocksPort/ControlPort
//
// **ControlPort authentication failed**
//
//	Error: control_auth_failed: AUTHENTICATE failed
//	Solution: Check authentication method and credentials
//	  For system Tor with cookie auth:
//	    auth, _, _ := tornago.ControlAuthFromTor("127.0.0.1:9051", 30*time.Second)
//	  Verify cookie file permissions:
//	    ls -l /run/tor/control.authcookie
//	  Add user to tor group if needed:
//	    sudo usermod -a -G debian-tor $USER
//
// **Requests timeout**
//
//	Error: timeout: context deadline exceeded
//	Solution: Increase timeouts for slow Tor connections
//	  .onion sites take 5-30 seconds to connect typically
//	  Use longer timeouts:
//	    tornago.WithClientDialTimeout(60*time.Second)
//	    tornago.WithClientRequestTimeout(120*time.Second)
//
// **Hidden Service not accessible**
//
//	Symptoms: .onion address times out, local server works
//	Solution: Wait for service to establish (30-60 seconds after creation)
//	  Access through Tor Browser or tornago client
//	  Verify local service is listening before creating hidden service
//
// **Checking error types**
//
//	resp, err := client.Do(req)
//	if err != nil {
//	    var torErr *tornago.TornagoError
//	    if errors.As(err, &torErr) {
//	        switch torErr.Kind {
//	        case tornago.ErrTimeout:
//	            // Increase timeout
//	        case tornago.ErrSocksDialFailed:
//	            // Check Tor connection
//	        case tornago.ErrHTTPFailed:
//	            // Handle HTTP error
//	        }
//	    }
//	}
//
// # Additional Documentation
//
// For detailed configuration examples and troubleshooting steps, see:
//   - doc/CONFIGURATION.md - Recommended settings for different use cases
//   - doc/TROUBLESHOOTING.md - Common issues and solutions
//   - examples/ directory - Working code examples
package tornago
