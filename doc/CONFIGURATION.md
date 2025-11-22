# Configuration Guide

This guide provides recommended configuration values for different use cases.

## Client Configuration

### Development Environment

For local testing and development:

```go
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
    tornago.WithClientDialTimeout(10*time.Second),
    tornago.WithClientRequestTimeout(30*time.Second),
)
```

### Production Environment

For production use with existing Tor daemon:

```go
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
    tornago.WithClientDialTimeout(30*time.Second),
    tornago.WithClientRequestTimeout(120*time.Second),
    tornago.WithClientRetryDelay(500*time.Millisecond),
    tornago.WithClientRetryMaxDelay(10*time.Second),
)
```

### With Metrics and Rate Limiting

For monitoring and controlling request rates:

```go
metrics := tornago.NewMetricsCollector()
rateLimiter := tornago.NewRateLimiter(5.0, 10) // 5 req/s, burst 10

clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
    tornago.WithClientRequestTimeout(60*time.Second),
    tornago.WithClientMetrics(metrics),
    tornago.WithClientRateLimiter(rateLimiter),
)
```

Check metrics:
```go
fmt.Printf("Total requests: %d\n", metrics.RequestCount())
fmt.Printf("Success rate: %.2f%%\n",
    float64(metrics.SuccessCount())/float64(metrics.RequestCount())*100)
fmt.Printf("Average latency: %v\n", metrics.AverageLatency())
```

### Web Scraping

For crawling multiple sites:

```go
metrics := tornago.NewMetricsCollector()
rateLimiter := tornago.NewRateLimiter(2.0, 5) // Conservative rate

clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
    tornago.WithClientDialTimeout(30*time.Second),
    tornago.WithClientRequestTimeout(120*time.Second), // Long timeout for slow sites
    tornago.WithClientMetrics(metrics),
    tornago.WithClientRateLimiter(rateLimiter),
)
```

## Tor Launch Configuration

### Development (Ephemeral Instance)

Launch temporary Tor daemon for testing:

```go
launchCfg, err := tornago.NewTorLaunchConfig(
    tornago.WithTorSocksAddr(":0"),     // Random available port
    tornago.WithTorControlAddr(":0"),   // Random available port
    tornago.WithTorStartupTimeout(60*time.Second),
    // DataDirectory defaults to temporary directory (auto-cleanup)
)

torProcess, err := tornago.StartTorDaemon(launchCfg)
if err != nil {
    log.Fatal(err)
}
defer torProcess.Stop()

fmt.Printf("SOCKS: %s\n", torProcess.SocksAddr())
fmt.Printf("Control: %s\n", torProcess.ControlAddr())
```

### Development (Persistent Data)

Keep Tor state between runs for faster startup:

```go
dataDir := filepath.Join(os.UserHomeDir(), ".cache", "tornago-dev")

launchCfg, err := tornago.NewTorLaunchConfig(
    tornago.WithTorSocksAddr(":9150"),
    tornago.WithTorControlAddr(":9151"),
    tornago.WithTorDataDirectory(dataDir),
    tornago.WithTorStartupTimeout(60*time.Second),
)
```

### Production

Use system Tor daemon instead of launching:

```go
// Don't launch Tor in production
// Use system service: sudo systemctl start tor

clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
    tornago.WithClientRequestTimeout(120*time.Second),
)
```

Configure system Tor (`/etc/tor/torrc`):
```
SocksPort 127.0.0.1:9050
ControlPort 127.0.0.1:9051
CookieAuthentication 1
DataDirectory /var/lib/tor
```

## Hidden Service Configuration

### Basic Hidden Service

Expose local HTTP server as .onion:

```go
hsCfg, err := tornago.NewHiddenServiceConfig(
    tornago.WithHiddenServicePort(80, 8080), // onion:80 -> local:8080
)

hs, err := controlClient.CreateHiddenService(ctx, hsCfg)
if err != nil {
    log.Fatal(err)
}
defer hs.Remove(ctx)

fmt.Printf("Onion address: http://%s\n", hs.OnionAddress())
```

### Persistent Hidden Service

Keep same .onion address across restarts:

```go
keyFile := "/path/to/persistent/hs.key"

var hsCfg tornago.HiddenServiceConfig
if _, err := os.Stat(keyFile); err == nil {
    // Load existing key
    hsCfg, err = tornago.NewHiddenServiceConfig(
        tornago.WithHiddenServicePrivateKeyFile(keyFile),
        tornago.WithHiddenServicePort(80, 8080),
    )
} else {
    // Create new key
    hsCfg, err = tornago.NewHiddenServiceConfig(
        tornago.WithHiddenServicePort(80, 8080),
    )
}

hs, err := controlClient.CreateHiddenService(ctx, hsCfg)
if err != nil {
    log.Fatal(err)
}

// Save key on first run
if _, err := os.Stat(keyFile); os.IsNotExist(err) {
    if err := hs.SavePrivateKey(keyFile); err != nil {
        log.Fatal(err)
    }
}
```

### Multiple Ports

Expose multiple services through one .onion:

```go
hsCfg, err := tornago.NewHiddenServiceConfig(
    tornago.WithHiddenServicePort(80, 8080),   // HTTP
    tornago.WithHiddenServicePort(443, 8443),  // HTTPS
)
```

Or use convenience helpers:

```go
hsCfg, err := tornago.NewHiddenServiceConfig(
    tornago.WithHiddenServiceHTTP(8080),   // Maps 80 -> 8080
    tornago.WithHiddenServiceHTTPS(8443),  // Maps 443 -> 8443
)
```

## Timeout Recommendations

Based on typical Tor performance:

### Dial Timeout
- **Development:** 10 seconds
- **Production:** 20-30 seconds
- **.onion sites:** 30-60 seconds

```go
tornago.WithClientDialTimeout(30*time.Second)
```

### Request Timeout
- **Simple requests:** 30-60 seconds
- **Large downloads:** 120-300 seconds
- **.onion sites:** 60-180 seconds

```go
tornago.WithClientRequestTimeout(120*time.Second)
```

### Startup Timeout
- **First launch:** 60-120 seconds
- **Cached state:** 30-60 seconds
- **Slow network:** 120+ seconds

```go
tornago.WithTorStartupTimeout(60*time.Second)
```

## Rate Limiting

### Conservative (Respectful Scraping)
```go
rateLimiter := tornago.NewRateLimiter(1.0, 3) // 1 req/s, burst 3
```

### Moderate (General Use)
```go
rateLimiter := tornago.NewRateLimiter(5.0, 10) // 5 req/s, burst 10
```

### Aggressive (High Volume)
```go
rateLimiter := tornago.NewRateLimiter(10.0, 20) // 10 req/s, burst 20
```

Note: Tor network capacity is limited. Excessive requests may degrade performance.

## Retry Configuration

### Conservative
```go
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr(addr),
    tornago.WithClientRetryDelay(1*time.Second),
    tornago.WithClientRetryMaxDelay(30*time.Second),
)
```

### Aggressive
```go
clientCfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr(addr),
    tornago.WithClientRetryDelay(100*time.Millisecond),
    tornago.WithClientRetryMaxDelay(5*time.Second),
)
```

## Complete Examples

### Production Web Scraper

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/nao1215/tornago"
)

func main() {
    // Metrics for monitoring
    metrics := tornago.NewMetricsCollector()

    // Rate limiting to avoid abuse
    rateLimiter := tornago.NewRateLimiter(2.0, 5)

    // Client configuration
    clientCfg, err := tornago.NewClientConfig(
        tornago.WithClientSocksAddr("127.0.0.1:9050"),
        tornago.WithClientDialTimeout(30*time.Second),
        tornago.WithClientRequestTimeout(120*time.Second),
        tornago.WithClientMetrics(metrics),
        tornago.WithClientRateLimiter(rateLimiter),
    )
    if err != nil {
        log.Fatal(err)
    }

    client, err := tornago.NewClient(clientCfg)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Use client for requests...
    // Periodically check metrics:
    log.Printf("Requests: %d, Success: %d, Errors: %d",
        metrics.RequestCount(),
        metrics.SuccessCount(),
        metrics.ErrorCount())
}
```

### Development Testing Setup

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/nao1215/tornago"
)

func main() {
    // Launch ephemeral Tor daemon
    launchCfg, err := tornago.NewTorLaunchConfig(
        tornago.WithTorSocksAddr(":0"),
        tornago.WithTorControlAddr(":0"),
        tornago.WithTorStartupTimeout(60*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }

    torProcess, err := tornago.StartTorDaemon(launchCfg)
    if err != nil {
        log.Fatal(err)
    }
    defer torProcess.Stop()

    log.Printf("Tor started: SOCKS=%s Control=%s",
        torProcess.SocksAddr(), torProcess.ControlAddr())

    // Create client
    clientCfg, err := tornago.NewClientConfig(
        tornago.WithClientSocksAddr(torProcess.SocksAddr()),
        tornago.WithClientRequestTimeout(30*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }

    client, err := tornago.NewClient(clientCfg)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Use client for testing...
}
```

## Configuration Validation

Always handle configuration errors:

```go
cfg, err := tornago.NewClientConfig(
    tornago.WithClientSocksAddr(""),  // Invalid: empty
)
if err != nil {
    var torErr *tornago.TornagoError
    if errors.As(err, &torErr) && torErr.Kind == tornago.ErrInvalidConfig {
        log.Printf("Configuration error: %s", torErr.Msg)
    }
    log.Fatal(err)
}
```

## Environment-Specific Configurations

### CI/CD Testing

Use fast timeouts and ephemeral instances:

```go
launchCfg, _ := tornago.NewTorLaunchConfig(
    tornago.WithTorSocksAddr(":0"),
    tornago.WithTorControlAddr(":0"),
    tornago.WithTorStartupTimeout(120*time.Second), // CI may be slow
)
```

### Docker Container

Mount persistent DataDirectory:

```go
launchCfg, _ := tornago.NewTorLaunchConfig(
    tornago.WithTorSocksAddr("127.0.0.1:9050"),
    tornago.WithTorControlAddr("127.0.0.1:9051"),
    tornago.WithTorDataDirectory("/var/lib/tor"),
    tornago.WithTorStartupTimeout(60*time.Second),
)
```

### High-Availability Service

Use system Tor with monitoring:

```go
// Monitor system Tor health
auth, _, err := tornago.ControlAuthFromTor("127.0.0.1:9051", 5*time.Second)
if err != nil {
    log.Fatal("Tor not available")
}

controlClient, _ := tornago.NewControlClient("127.0.0.1:9051", auth, 5*time.Second)
defer controlClient.Close()

if err := controlClient.Authenticate(); err != nil {
    log.Fatal("Cannot authenticate to Tor")
}

// Tor is healthy, create client
clientCfg, _ := tornago.NewClientConfig(
    tornago.WithClientSocksAddr("127.0.0.1:9050"),
    tornago.WithClientRequestTimeout(120*time.Second),
)
```
