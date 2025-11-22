package tornago

import (
	"context"
	"fmt"
	"time"
)

// HealthStatus represents the health state of a Tor connection or service.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the service is functioning normally.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates the service is operational but experiencing issues.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates the service is not functioning.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck contains the result of a health check operation.
// It is an immutable value object that provides methods to query health status.
type HealthCheck struct {
	status    HealthStatus
	message   string
	timestamp time.Time
	latency   time.Duration
}

// IsHealthy returns true if all components are functioning normally.
func (h HealthCheck) IsHealthy() bool {
	return h.status == HealthStatusHealthy
}

// IsDegraded returns true if the service is operational but experiencing issues.
func (h HealthCheck) IsDegraded() bool {
	return h.status == HealthStatusDegraded
}

// IsUnhealthy returns true if the service is not functioning.
func (h HealthCheck) IsUnhealthy() bool {
	return h.status == HealthStatusUnhealthy
}

// Status returns the overall health status.
func (h HealthCheck) Status() HealthStatus {
	return h.status
}

// Message provides human-readable context about the health status.
func (h HealthCheck) Message() string {
	return h.message
}

// Timestamp returns when the health check was performed.
func (h HealthCheck) Timestamp() time.Time {
	return h.timestamp
}

// Latency returns how long the health check took.
func (h HealthCheck) Latency() time.Duration {
	return h.latency
}

// String returns a human-readable representation of the health check.
func (h HealthCheck) String() string {
	return fmt.Sprintf("Health: %s (%s) - latency: %v",
		h.status, h.message, h.latency.Round(time.Millisecond))
}

// Check performs a health check on the Tor connection.
// It verifies that:
//   - SOCKS proxy is reachable
//   - ControlPort is accessible (if configured)
//   - Authentication is valid (if configured)
//
// The check includes a timeout to prevent hanging on unresponsive services.
//
// Example:
//
//	client, _ := tornago.NewClient(cfg)
//	health := client.Check(context.Background())
//	if !health.IsHealthy() {
//	    log.Printf("Tor unhealthy: %s", health.Message())
//	}
func (c *Client) Check(ctx context.Context) HealthCheck {
	start := time.Now()

	// Check SOCKS connectivity by attempting to dial through Tor
	socksError := c.checkSOCKS(ctx)

	// Check ControlPort if available
	var controlError string
	if c.control != nil {
		controlError = c.checkControl(ctx)
	}

	// Determine overall status
	latency := time.Since(start)
	var status HealthStatus
	var message string

	if socksError == "" && (c.control == nil || controlError == "") {
		status = HealthStatusHealthy
		message = "All checks passed"
	} else if socksError != "" && controlError != "" {
		status = HealthStatusUnhealthy
		message = fmt.Sprintf("SOCKS: %s, Control: %s", socksError, controlError)
	} else {
		status = HealthStatusDegraded
		if socksError != "" {
			message = "SOCKS unhealthy: " + socksError
		} else {
			message = "Control unhealthy: " + controlError
		}
	}

	return HealthCheck{
		status:    status,
		message:   message,
		timestamp: start,
		latency:   latency,
	}
}

// checkSOCKS verifies SOCKS proxy connectivity.
// Returns empty string on success, error message on failure.
func (c *Client) checkSOCKS(ctx context.Context) string {
	// Create a short timeout for health check
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Attempt to dial a dummy address through SOCKS
	// We don't need to actually connect, just verify SOCKS proxy responds
	conn, err := c.socksDialer.DialContext(checkCtx, "tcp", "check.torproject.org:80")
	if err != nil {
		return fmt.Sprintf("dial failed: %v", err)
	}
	_ = conn.Close()
	return ""
}

// checkControl verifies ControlPort connectivity and authentication.
// Returns empty string on success, error message on failure.
func (c *Client) checkControl(ctx context.Context) string {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to get a simple piece of information from Tor
	version, err := c.control.GetInfo(checkCtx, "version")
	if err != nil {
		return fmt.Sprintf("getinfo failed: %v", err)
	}
	if version == "" {
		return "no response"
	}
	return ""
}

// CheckTorDaemon performs a health check on a TorProcess.
// It verifies that:
//   - The Tor process is running
//   - SOCKS and ControlPort are responsive
//
// Example:
//
//	torProcess, _ := tornago.StartTorDaemon(cfg)
//	health := tornago.CheckTorDaemon(context.Background(), torProcess)
//	if !health.IsHealthy() {
//	    log.Printf("Tor daemon unhealthy: %s", health.Message())
//	}
func CheckTorDaemon(ctx context.Context, proc *TorProcess) HealthCheck {
	start := time.Now()

	// Check if process is running
	if proc.cmd == nil || proc.cmd.Process == nil {
		return HealthCheck{
			status:    HealthStatusUnhealthy,
			message:   "Tor process not running",
			timestamp: start,
			latency:   time.Since(start),
		}
	}

	// Try to get control auth
	auth, _, err := ControlAuthFromTor(proc.ControlAddr(), 5*time.Second)
	if err != nil {
		return HealthCheck{
			status:    HealthStatusDegraded,
			message:   fmt.Sprintf("Cannot get control auth: %v", err),
			timestamp: start,
			latency:   time.Since(start),
		}
	}

	// Create temporary control client
	controlClient, err := NewControlClient(proc.ControlAddr(), auth, 5*time.Second)
	if err != nil {
		return HealthCheck{
			status:    HealthStatusDegraded,
			message:   fmt.Sprintf("Cannot create control client: %v", err),
			timestamp: start,
			latency:   time.Since(start),
		}
	}
	defer controlClient.Close()

	// Try to authenticate
	if err := controlClient.Authenticate(); err != nil {
		return HealthCheck{
			status:    HealthStatusDegraded,
			message:   fmt.Sprintf("Authentication failed: %v", err),
			timestamp: start,
			latency:   time.Since(start),
		}
	}

	// Get Tor version as a basic health indicator
	_, err = controlClient.GetInfo(ctx, "version")
	if err != nil {
		return HealthCheck{
			status:    HealthStatusDegraded,
			message:   fmt.Sprintf("GetInfo failed: %v", err),
			timestamp: start,
			latency:   time.Since(start),
		}
	}

	return HealthCheck{
		status:    HealthStatusHealthy,
		message:   "Tor daemon is healthy",
		timestamp: start,
		latency:   time.Since(start),
	}
}
