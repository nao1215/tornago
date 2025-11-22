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
// # API Design Principles
//
// **Method Naming Conventions**
//
// tornago follows a consistent naming convention to distinguish between different
// types of verification operations:
//
//   - Check*() methods perform internal health checks and status verification
//
//   - Check() - Verifies SOCKS and ControlPort connectivity
//
//   - CheckDNSLeak() - Detects if DNS queries are leaking outside Tor
//
//   - CheckTorDaemon() - Checks if the Tor process is running properly
//
//   - These methods are faster but rely on internal heuristics
//
//   - Verify*() methods use external validation via third-party services
//
//   - VerifyTorConnection() - Confirms Tor usage via check.torproject.org
//
//   - These methods are more authoritative but depend on external services
//
// This distinction helps you choose the appropriate method:
//   - Use Check*() for quick health monitoring and internal validation
//   - Use Verify*() when you need authoritative external confirmation
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
// **With Observability (Logging and Health Checks)**
//
//	// Structured logging
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	slogAdapter := tornago.NewSlogAdapter(logger)
//
//	clientCfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	    tornago.WithClientLogger(slogAdapter),
//	)
//
//	client, _ := tornago.NewClient(clientCfg)
//
//	// Health check
//	health := client.Check(context.Background())
//	if !health.IsHealthy() {
//	    log.Printf("Client unhealthy: %s", health.Message())
//	}
//
// **Hidden Service with Persistent Key**
//
//	const keyPath = "/var/lib/myapp/onion.pem"
//
//	// Try to load existing key
//	privateKey, err := tornago.LoadPrivateKey(keyPath)
//	if err != nil {
//	    // First run: create new service
//	    hsCfg, _ := tornago.NewHiddenServiceConfig(
//	        tornago.WithHiddenServicePort(80, 8080),
//	    )
//	    hs, _ := controlClient.CreateHiddenService(ctx, hsCfg)
//
//	    // Save key for next time
//	    tornago.SavePrivateKey(keyPath, hs.PrivateKey())
//	    os.Chmod(keyPath, 0600)
//	} else {
//	    // Reuse existing key - same .onion address
//	    hsCfg, _ := tornago.NewHiddenServiceConfig(
//	        tornago.WithHiddenServicePrivateKey(privateKey),
//	        tornago.WithHiddenServicePort(80, 8080),
//	    )
//	    hs, _ := controlClient.CreateHiddenService(ctx, hsCfg)
//	}
//
// # Security Best Practices
//
// **Connection Verification**
//
// Always verify that your application is routing traffic through Tor:
//
//	status, err := client.VerifyTorConnection(ctx)
//	if err != nil {
//	    log.Fatalf("Verification failed: %v", err)
//	}
//	if !status.IsUsingTor() {
//	    log.Fatalf("CRITICAL: Traffic is NOT going through Tor!")
//	}
//
// When to verify:
//   - On application startup
//   - After configuration changes
//   - Periodically in long-running services
//   - Before handling sensitive operations
//
// **DNS Leak Prevention**
//
// Check if DNS queries are leaking outside Tor:
//
//	leakCheck, err := client.CheckDNSLeak(ctx)
//	if err != nil {
//	    log.Fatalf("DNS leak check failed: %v", err)
//	}
//	if leakCheck.HasLeak() {
//	    log.Fatalf("WARNING: DNS leak detected! IPs: %v", leakCheck.ResolvedIPs())
//	}
//
// DNS leaks reveal which domains you're accessing to your ISP or DNS provider.
//
// **Hidden Service Private Key Management**
//
// Private keys determine your .onion address. Keep them secure:
//
//	// File permissions
//	sudo chmod 600 /var/lib/myapp/onion.pem
//	sudo chown myapp:myapp /var/lib/myapp/onion.pem
//
//	// Encrypted backups
//	openssl enc -aes-256-cbc -salt \
//	    -in /var/lib/myapp/onion.pem \
//	    -out onion.pem.enc
//
// Best practices:
//   - Store keys in secure directory with restricted permissions (chmod 600)
//   - Keep encrypted backups in separate physical location
//   - Test restoration regularly
//   - Use SELinux/AppArmor for additional protection
//   - Monitor key file access with audit logs
//
// **Common Security Pitfalls**
//
// 1. Using HTTP instead of HTTPS:
//   - Exit nodes can see unencrypted HTTP traffic
//   - Always use HTTPS for end-to-end encryption
//
// 2. Leaking metadata:
//   - Remove identifying headers (User-Agent, X-Forwarded-For)
//   - Minimize timestamps and other identifying information
//
// 3. Circuit reuse correlation:
//   - Rotate circuits for sensitive operations using NewIdentity()
//   - Wait 5-10 seconds after rotation for new circuit to build
//
// 4. Insufficient timeouts:
//   - Use minimum 30s dial timeout, 60-120s request timeout
//   - .onion sites require even longer timeouts (60s+ dial, 120s+ request)
//
// 5. Not verifying .onion addresses:
//   - Hardcode trusted .onion addresses
//   - Verify addresses through trusted channels to avoid phishing
//
// **Client Authentication for Hidden Services**
//
// Restrict hidden service access to authorized clients:
//
//	// Server side
//	auth := tornago.HiddenServiceAuth{
//	    ClientName: "authorized-client-1",
//	    PublicKey:  "descriptor:x25519:AAAA...base64-public-key",
//	}
//	hsCfg, _ := tornago.NewHiddenServiceConfig(
//	    tornago.WithHiddenServiceClientAuth(auth),
//	)
//
// Generate x25519 key pairs:
//
//	openssl genpkey -algorithm x25519 -out private.pem
//	openssl pkey -in private.pem -pubout -out public.pem
//	openssl pkey -in public.pem -pubin -outform DER | tail -c 32 | base64
//
// Security benefits:
//   - Only authorized clients can discover the service
//   - Protects against descriptor enumeration attacks
//   - Provides end-to-end authentication
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
// **Testing Tor connection manually**
//
//	# Test SOCKS proxy
//	curl --socks5 127.0.0.1:9050 https://check.torproject.org/api/ip
//
//	# Test .onion access
//	curl --socks5 127.0.0.1:9050 http://your-address.onion
//
// # Rate Limiting Recommendations
//
// Tor network capacity is limited. Use appropriate rate limits:
//
//	// Conservative (respectful scraping)
//	rateLimiter := tornago.NewRateLimiter(1.0, 3)  // 1 req/s, burst 3
//
//	// Moderate (general use)
//	rateLimiter := tornago.NewRateLimiter(5.0, 10)  // 5 req/s, burst 10
//
//	// Aggressive (high volume)
//	rateLimiter := tornago.NewRateLimiter(10.0, 20)  // 10 req/s, burst 20
//
// Excessive requests may degrade Tor network performance for everyone.
//
// # Environment-Specific Configurations
//
// **CI/CD Testing**
//
// Use fast timeouts and ephemeral instances:
//
//	launchCfg, _ := tornago.NewTorLaunchConfig(
//	    tornago.WithTorSocksAddr(":0"),
//	    tornago.WithTorControlAddr(":0"),
//	    tornago.WithTorStartupTimeout(120*time.Second), // CI may be slow
//	)
//
// **Docker Container**
//
// Mount persistent DataDirectory:
//
//	launchCfg, _ := tornago.NewTorLaunchConfig(
//	    tornago.WithTorSocksAddr("127.0.0.1:9050"),
//	    tornago.WithTorControlAddr("127.0.0.1:9051"),
//	    tornago.WithTorDataDirectory("/var/lib/tor"),
//	)
//
// **High-Availability Service**
//
// Use system Tor with health monitoring:
//
//	// Verify Tor availability
//	auth, _, err := tornago.ControlAuthFromTor("127.0.0.1:9051", 5*time.Second)
//	if err != nil {
//	    log.Fatal("Tor not available")
//	}
//
//	client, _ := tornago.NewClient(clientCfg)
//	health := client.Check(ctx)
//	if !health.IsHealthy() {
//	    log.Fatalf("Tor unhealthy: %s", health.Message())
//	}
//
// # Additional Resources
//
// For working code examples, see the examples/ directory:
//   - examples/simple_client - Basic HTTP requests through Tor
//   - examples/onion_client - Accessing .onion sites
//   - examples/onion_server - Creating Hidden Services
//   - examples/existing_tor - Connecting to system Tor daemon
//   - examples/circuit_rotation - Rotating circuits to change exit IP
//   - examples/error_handling - Proper error handling patterns
//   - examples/metrics_ratelimit - Metrics collection and rate limiting
//   - examples/persistent_onion - Hidden Service with persistent key
//   - examples/observability - Structured logging, metrics, and health checks
//   - examples/security - Tor connection verification and DNS leak detection
//
// Complete API documentation: https://pkg.go.dev/github.com/nao1215/tornago
package tornago
