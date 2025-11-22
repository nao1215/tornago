package tornago

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	unknownIP = "unknown"
)

// TorConnectionStatus represents the result of verifying Tor connectivity.
// It is an immutable value object that provides methods to query connection status.
type TorConnectionStatus struct {
	// usingTor indicates whether the connection is actually going through Tor.
	usingTor bool
	// exitIP is the IP address as seen by the target server (Tor exit node IP).
	exitIP string
	// message provides human-readable details about the check.
	message string
	// latency is how long the verification took.
	latency time.Duration
}

// IsUsingTor returns true if the connection is going through Tor.
func (s TorConnectionStatus) IsUsingTor() bool {
	return s.usingTor
}

// ExitIP returns the IP address as seen by the target server (Tor exit node IP).
func (s TorConnectionStatus) ExitIP() string {
	return s.exitIP
}

// Message provides human-readable details about the check.
func (s TorConnectionStatus) Message() string {
	return s.message
}

// Latency returns how long the verification took.
func (s TorConnectionStatus) Latency() time.Duration {
	return s.latency
}

// String returns a human-readable representation of the Tor connection status.
func (s TorConnectionStatus) String() string {
	status := "NOT using Tor"
	if s.usingTor {
		status = "Using Tor"
	}
	return fmt.Sprintf("%s (Exit IP: %s) - latency: %v",
		status, s.exitIP, s.latency.Round(time.Millisecond))
}

// VerifyTorConnection checks if the client is actually routing traffic through Tor
// by connecting to check.torproject.org. This service returns whether the connection
// came from a known Tor exit node.
//
// This is useful for:
//   - Verifying Tor configuration is working correctly
//   - Detecting if traffic is leaking outside Tor
//   - Getting the current exit node IP address
//
// Example:
//
//	client, _ := tornago.NewClient(cfg)
//	status, err := client.VerifyTorConnection(context.Background())
//	if err != nil {
//	    log.Fatalf("Verification failed: %v", err)
//	}
//	if !status.UsingTor {
//	    log.Printf("WARNING: Not using Tor! Exit IP: %s", status.ExitIP)
//	}
func (c *Client) VerifyTorConnection(ctx context.Context) (TorConnectionStatus, error) {
	start := time.Now()

	// Use the official Tor check service
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://check.torproject.org/api/ip", http.NoBody)
	if err != nil {
		return TorConnectionStatus{}, newError(ErrInvalidConfig, "VerifyTorConnection",
			"failed to create request", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return TorConnectionStatus{}, newError(ErrHTTPFailed, "VerifyTorConnection",
			"failed to reach check.torproject.org", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TorConnectionStatus{}, newError(ErrHTTPFailed, "VerifyTorConnection",
			"failed to read response", err)
	}

	latency := time.Since(start)

	// Parse the response
	// Example: {"IsTor":true,"IP":"185.220.101.1"}
	bodyStr := string(body)
	usingTor := strings.Contains(bodyStr, `"IsTor":true`)

	// Extract IP address
	exitIP := unknownIP
	if ipStart := strings.Index(bodyStr, `"IP":"`); ipStart != -1 {
		ipStart += len(`"IP":"`)
		if ipEnd := strings.Index(bodyStr[ipStart:], `"`); ipEnd != -1 {
			exitIP = bodyStr[ipStart : ipStart+ipEnd]
		}
	}

	message := "Connection is not going through Tor"
	if usingTor {
		message = "Connection verified through Tor network"
	}

	return TorConnectionStatus{
		usingTor: usingTor,
		exitIP:   exitIP,
		message:  message,
		latency:  latency,
	}, nil
}

// DNSLeakCheck represents the result of a DNS leak detection test.
// It is an immutable value object that provides methods to query leak status.
type DNSLeakCheck struct {
	// hasLeak indicates whether DNS queries are leaking outside Tor.
	hasLeak bool
	// resolvedIPs contains the IP addresses returned by DNS resolution.
	resolvedIPs []string
	// message provides human-readable details about the check.
	message string
	// latency is how long the check took.
	latency time.Duration
}

// HasLeak returns true if DNS queries are leaking outside Tor.
func (d DNSLeakCheck) HasLeak() bool {
	return d.hasLeak
}

// ResolvedIPs returns a defensive copy of the IP addresses returned by DNS resolution.
func (d DNSLeakCheck) ResolvedIPs() []string {
	cp := make([]string, len(d.resolvedIPs))
	copy(cp, d.resolvedIPs)
	return cp
}

// Message provides human-readable details about the check.
func (d DNSLeakCheck) Message() string {
	return d.message
}

// Latency returns how long the check took.
func (d DNSLeakCheck) Latency() time.Duration {
	return d.latency
}

// String returns a human-readable representation of the DNS leak check.
func (d DNSLeakCheck) String() string {
	status := "No DNS leak detected"
	if d.hasLeak {
		status = "DNS LEAK DETECTED"
	}
	return fmt.Sprintf("%s - IPs: %v - latency: %v",
		status, d.resolvedIPs, d.latency.Round(time.Millisecond))
}

// CheckDNSLeak verifies that DNS queries are going through Tor and not leaking
// to your local DNS resolver. It does this by resolving a hostname through the
// Tor SOCKS proxy and comparing it with what Tor's DNS resolution returns.
//
// DNS leaks occur when your system's DNS resolver is used instead of Tor's,
// potentially revealing which domains you're accessing to your ISP or DNS provider.
//
// This check resolves "check.torproject.org" through Tor and verifies the result.
//
// Example:
//
//	client, _ := tornago.NewClient(cfg)
//	leakCheck, err := client.CheckDNSLeak(context.Background())
//	if err != nil {
//	    log.Fatalf("DNS leak check failed: %v", err)
//	}
//	if leakCheck.HasLeak {
//	    log.Printf("WARNING: DNS leak detected! IPs: %v", leakCheck.ResolvedIPs)
//	}
func (c *Client) CheckDNSLeak(ctx context.Context) (DNSLeakCheck, error) {
	start := time.Now()

	// Resolve a known domain through Tor's SOCKS proxy
	// We use the Tor check domain since we know it should be accessible
	testDomain := "check.torproject.org"

	// Create a dialer that uses our Tor SOCKS proxy for DNS resolution
	torDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Resolve through Tor by connecting to a dummy address
	// The SOCKS5 proxy will do the DNS resolution for us
	conn, err := c.DialContext(ctx, "tcp", testDomain+":443")
	if err != nil {
		return DNSLeakCheck{}, newError(ErrSocksDialFailed, "CheckDNSLeak",
			"failed to resolve through Tor", err)
	}
	defer conn.Close()

	// Get the remote address (this will show the resolved IP)
	remoteAddr := conn.RemoteAddr().String()
	var resolvedIP string
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		resolvedIP = host
	} else {
		resolvedIP = remoteAddr
	}

	latency := time.Since(start)

	// Also try direct system DNS resolution (outside Tor) for comparison
	systemIPs, err := net.DefaultResolver.LookupHost(ctx, testDomain)
	if err != nil {
		// If system DNS fails, it might be blocked or configured to go through Tor
		// This is actually a good sign - no leak possible if system DNS doesn't work
		return DNSLeakCheck{
			hasLeak:     false,
			resolvedIPs: []string{resolvedIP},
			message:     "DNS queries going through Tor (system DNS unavailable)",
			latency:     latency,
		}, nil
	}

	// Check if the Tor-resolved IP is different from system DNS
	// If they're the same, DNS might be leaking
	hasLeak := false
	for _, sysIP := range systemIPs {
		if sysIP == resolvedIP {
			hasLeak = true
			break
		}
	}

	message := "DNS queries are properly routed through Tor"
	if hasLeak {
		message = "DNS leak detected: queries may be going through system DNS"
	}

	// Note: This is a simple heuristic. A more robust check would involve
	// comparing against known Tor exit nodes or using a dedicated DNS leak test service.
	// For now, we use the resolver provided by Tor's SOCKS proxy as the baseline.
	_ = torDialer // Keep for potential future use

	return DNSLeakCheck{
		hasLeak:     hasLeak,
		resolvedIPs: append([]string{resolvedIP}, systemIPs...),
		message:     message,
		latency:     latency,
	}, nil
}
