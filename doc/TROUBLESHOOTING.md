# Troubleshooting Guide

This guide covers common issues and their solutions when using tornago.

## Installation Issues

### Tor binary not found

**Error:**
```
TorDaemon: tor_binary_not_found: tor executable not found in PATH
```

**Solution:**
Install Tor for your system:

```bash
# Ubuntu/Debian
sudo apt update && sudo apt install tor

# Fedora/RHEL
sudo dnf install tor

# Arch Linux
sudo pacman -S tor

# macOS
brew install tor
```

Verify installation:
```bash
tor --version
```

### Tor fails to start

**Error:**
```
TorDaemon: tor_launch_failed: failed to parse Tor bootstrap: context deadline exceeded
```

**Common causes:**
1. Startup timeout too short
2. Port already in use
3. Permission issues with DataDirectory

**Solutions:**

Increase timeout:
```go
launchCfg, err := tornago.NewTorLaunchConfig(
    tornago.WithTorStartupTimeout(120*time.Second), // Increase from default
)
```

Use random ports to avoid conflicts:
```go
launchCfg, err := tornago.NewTorLaunchConfig(
    tornago.WithTorSocksAddr(":0"),     // Random port
    tornago.WithTorControlAddr(":0"),   // Random port
)
```

Check directory permissions:
```bash
ls -la ~/.cache/tornago/  # Default DataDirectory
```

## Connection Issues

### Cannot connect to existing Tor daemon

**Error:**
```
ControlClient: control_request_failed: failed to dial ControlPort
```

**Solution:**
Check if Tor is running:
```bash
# Check process
ps aux | grep tor

# Check ports
sudo netstat -tlnp | grep tor
# or
sudo ss -tlnp | grep tor
```

Default Tor configuration (Debian/Ubuntu):
- SocksPort: `127.0.0.1:9050`
- ControlPort: `127.0.0.1:9051`

Verify your Tor configuration (`/etc/tor/torrc`):
```
SocksPort 127.0.0.1:9050
ControlPort 127.0.0.1:9051
CookieAuthentication 1
```

### SOCKS dial failed

**Error:**
```
Client: socks_dial_failed: failed to connect to SOCKS proxy
```

**Causes:**
1. Wrong SOCKS address
2. Tor not running
3. Firewall blocking connection

**Solution:**
Verify SOCKS address matches Tor configuration:
```go
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"), // Match your Tor config
)
```

### Connection timeout

**Error:**
```
Client: timeout: context deadline exceeded
```

**Solution:**
Increase timeouts for slow Tor connections:
```go
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientDialTimeout(30*time.Second),
    tornago.WithClientRequestTimeout(120*time.Second),
)
```

Note: .onion sites typically take longer to connect than clearnet sites.

## Authentication Issues

### ControlPort authentication failed

**Error:**
```
ControlClient: control_auth_failed: AUTHENTICATE failed
```

**Causes:**
1. Wrong authentication method
2. Incorrect password
3. Cookie file not readable

**Solutions:**

For system Tor (cookie auth):
```go
auth, _, err := tornago.ControlAuthFromTor("127.0.0.1:9051", 30*time.Second)
if err != nil {
    log.Fatal(err)
}
```

For password auth:
```go
auth := tornago.ControlAuthFromPassword("your_password")
```

Check cookie file permissions:
```bash
# Default location
ls -l /run/tor/control.authcookie
# or
ls -l /var/lib/tor/control_auth_cookie
```

Add your user to the tor group if needed:
```bash
sudo usermod -a -G debian-tor $USER
# Log out and back in for group changes to take effect
```

## Hidden Service Issues

### Hidden service creation fails

**Error:**
```
ControlClient: hidden_service_failed: ADD_ONION returned error
```

**Causes:**
1. Port already in use
2. Invalid configuration
3. ControlPort not authenticated

**Solution:**
Ensure local service is running before creating hidden service:
```go
// Start local HTTP server first
listener, err := net.Listen("tcp", "127.0.0.1:8080")
if err != nil {
    log.Fatal(err)
}

// Then create hidden service
hsCfg, _ := tornago.NewHiddenServiceConfig(
    tornago.WithHiddenServicePort(80, 8080), // onion:80 -> local:8080
)
```

### Cannot access .onion address

**Symptoms:**
- Local server works (`http://127.0.0.1:8080`)
- .onion address times out

**Causes:**
1. Hidden service not fully established (takes 30-60 seconds)
2. Accessing from non-Tor browser
3. Firewall blocking Tor traffic

**Solution:**
Wait for service to establish:
```go
hs, err := controlClient.CreateHiddenService(ctx, hsCfg)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Hidden service created: %s\n", hs.OnionAddress())
fmt.Println("Waiting for service to establish...")
time.Sleep(30 * time.Second) // Give time for circuits to build
```

Access using Tor Browser or through tornago client.

## Performance Issues

### Requests are very slow

**Expected behavior:**
Tor adds latency due to routing through multiple relays.

- Clearnet via Tor: 2-10 seconds typical
- .onion sites: 5-30 seconds typical

**Optimization:**
```go
// Increase retry attempts for flaky connections
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr(addr),
    tornago.WithClientRetryDelay(500*time.Millisecond),
    tornago.WithClientRetryMaxDelay(10*time.Second),
)
```

Use circuit rotation sparingly (creates new circuits):
```go
// Don't call this too frequently
controlClient.NewIdentity(ctx)
time.Sleep(5 * time.Second) // Wait for new circuit
```

### High memory usage

**Cause:**
Multiple concurrent connections or long-running daemon.

**Solution:**
Close clients when done:
```go
client, err := tornago.NewClient(cfg)
if err != nil {
    log.Fatal(err)
}
defer client.Close() // Important: always close

// Use the client...
```

Stop Tor daemon when no longer needed:
```go
torProcess, err := tornago.StartTorDaemon(cfg)
if err != nil {
    log.Fatal(err)
}
defer torProcess.Stop() // Cleanup
```

## Configuration Issues

### Invalid configuration error

**Error:**
```
invalid_config: <specific message>
```

**Common mistakes:**

Missing required options:
```go
// Wrong - no SOCKS address
cfg, err := tornago.NewClientConfig()

// Correct
cfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
)
```

Invalid port mapping:
```go
// Wrong - virtual and target ports swapped
tornago.WithHiddenServicePort(8080, 80)

// Correct - virtual:80 -> target:8080
tornago.WithHiddenServicePort(80, 8080)
```

## Error Handling

### Checking error types

Use `errors.As` and check `Kind` field:
```go
resp, err := client.Do(req)
if err != nil {
    var torErr *tornago.TornagoError
    if errors.As(err, &torErr) {
        switch torErr.Kind {
        case tornago.ErrTimeout:
            // Handle timeout
        case tornago.ErrHTTPFailed:
            // Handle HTTP error
        case tornago.ErrSocksDialFailed:
            // Handle connection error
        default:
            // Handle other errors
        }
    }
}
```

## Debug Tips

### Enable verbose logging

Tor daemon logs can help diagnose issues. Check stdout/stderr from StartTorDaemon.

### Test Tor connection manually

```bash
# Test SOCKS proxy
curl --socks5 127.0.0.1:9050 https://check.torproject.org/api/ip

# Test .onion access
curl --socks5 127.0.0.1:9050 http://your-address.onion
```

### Verify Tor is working

```go
req, _ := http.NewRequestWithContext(
    context.Background(),
    http.MethodGet,
    "https://check.torproject.org/api/ip",
    http.NoBody,
)

resp, err := client.Do(req)
if err != nil {
    log.Fatal(err)
}
defer resp.Body.Close()

body, _ := io.ReadAll(resp.Body)
fmt.Printf("Response: %s\n", body)
// Should show: {"IsTor":true,...}
```

## Getting Help

If you encounter an issue not covered here:

1. Check existing examples in `examples/` directory
2. Review API documentation: https://pkg.go.dev/github.com/nao1215/tornago
3. Search existing issues: https://github.com/nao1215/tornago/issues
4. Create a new issue with:
   - tornago version
   - Tor version (`tor --version`)
   - Operating system
   - Minimal code to reproduce
   - Complete error message
