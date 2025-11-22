package tornago

import (
	"context"
	"testing"
	"time"
)

func TestHealthStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status HealthStatus
		want   string
	}{
		{
			name:   "should return healthy status string",
			status: HealthStatusHealthy,
			want:   "healthy",
		},
		{
			name:   "should return degraded status string",
			status: HealthStatusDegraded,
			want:   "degraded",
		},
		{
			name:   "should return unhealthy status string",
			status: HealthStatusUnhealthy,
			want:   "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if string(tt.status) != tt.want {
				t.Errorf("HealthStatus = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

func TestHealthCheckAccessors(t *testing.T) {
	t.Parallel()

	now := time.Now()
	latency := 100 * time.Millisecond

	hc := HealthCheck{
		status:    HealthStatusHealthy,
		message:   "test message",
		timestamp: now,
		latency:   latency,
	}

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "should return correct status",
			test: func(t *testing.T) {
				t.Helper()
				t.Parallel()
				if hc.Status() != HealthStatusHealthy {
					t.Errorf("Status() = %v, want %v", hc.Status(), HealthStatusHealthy)
				}
			},
		},
		{
			name: "should return correct message",
			test: func(t *testing.T) {
				t.Helper()
				t.Parallel()
				if hc.Message() != "test message" {
					t.Errorf("Message() = %v, want %v", hc.Message(), "test message")
				}
			},
		},
		{
			name: "should return correct timestamp",
			test: func(t *testing.T) {
				t.Helper()
				t.Parallel()
				if !hc.Timestamp().Equal(now) {
					t.Errorf("Timestamp() = %v, want %v", hc.Timestamp(), now)
				}
			},
		},
		{
			name: "should return correct latency",
			test: func(t *testing.T) {
				t.Helper()
				t.Parallel()
				if hc.Latency() != latency {
					t.Errorf("Latency() = %v, want %v", hc.Latency(), latency)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestHealthCheckQueryMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		status      HealthStatus
		isHealthy   bool
		isDegraded  bool
		isUnhealthy bool
	}{
		{
			name:        "should identify healthy status",
			status:      HealthStatusHealthy,
			isHealthy:   true,
			isDegraded:  false,
			isUnhealthy: false,
		},
		{
			name:        "should identify degraded status",
			status:      HealthStatusDegraded,
			isHealthy:   false,
			isDegraded:  true,
			isUnhealthy: false,
		},
		{
			name:        "should identify unhealthy status",
			status:      HealthStatusUnhealthy,
			isHealthy:   false,
			isDegraded:  false,
			isUnhealthy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hc := HealthCheck{status: tt.status}

			if hc.IsHealthy() != tt.isHealthy {
				t.Errorf("IsHealthy() = %v, want %v", hc.IsHealthy(), tt.isHealthy)
			}
			if hc.IsDegraded() != tt.isDegraded {
				t.Errorf("IsDegraded() = %v, want %v", hc.IsDegraded(), tt.isDegraded)
			}
			if hc.IsUnhealthy() != tt.isUnhealthy {
				t.Errorf("IsUnhealthy() = %v, want %v", hc.IsUnhealthy(), tt.isUnhealthy)
			}
		})
	}
}

func TestHealthCheckString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		hc      HealthCheck
		wantStr string
	}{
		{
			name: "should format healthy status",
			hc: HealthCheck{
				status:  HealthStatusHealthy,
				message: "All systems operational",
				latency: 50 * time.Millisecond,
			},
			wantStr: "Health: healthy (All systems operational) - latency: 50ms",
		},
		{
			name: "should format degraded status",
			hc: HealthCheck{
				status:  HealthStatusDegraded,
				message: "SOCKS unhealthy: connection timeout",
				latency: 150 * time.Millisecond,
			},
			wantStr: "Health: degraded (SOCKS unhealthy: connection timeout) - latency: 150ms",
		},
		{
			name: "should format unhealthy status",
			hc: HealthCheck{
				status:  HealthStatusUnhealthy,
				message: "SOCKS: dial failed, Control: auth failed",
				latency: 1 * time.Second,
			},
			wantStr: "Health: unhealthy (SOCKS: dial failed, Control: auth failed) - latency: 1s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.hc.String()
			if got != tt.wantStr {
				t.Errorf("String() = %v, want %v", got, tt.wantStr)
			}
		})
	}
}

func TestClientCheckWithInvalidSOCKS(t *testing.T) {
	t.Parallel()

	// Create client with invalid SOCKS address
	cfg, err := NewClientConfig(
		WithClientSocksAddr("127.0.0.1:1"), // Port 1 should not be accessible
		WithClientDialTimeout(1*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClientConfig() error = %v", err)
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	health := client.Check(ctx)

	// Should be degraded (SOCKS unhealthy, no control port)
	if health.Status() != HealthStatusDegraded {
		t.Errorf("Check() status = %v, want %v (message: %s)",
			health.Status(), HealthStatusDegraded, health.Message())
	}

	if health.IsDegraded() != true {
		t.Error("IsDegraded() = false, want true")
	}
}

func TestCheckTorDaemonWithNilProcess(t *testing.T) {
	t.Parallel()

	proc := &TorProcess{
		cmd:     nil,
		process: nil,
	}

	ctx := context.Background()
	health := CheckTorDaemon(ctx, proc)

	if health.Status() != HealthStatusUnhealthy {
		t.Errorf("CheckTorDaemon() status = %v, want %v", health.Status(), HealthStatusUnhealthy)
	}

	if !health.IsUnhealthy() {
		t.Error("IsUnhealthy() = false, want true")
	}

	if health.Message() != "Tor process not running" {
		t.Errorf("CheckTorDaemon() message = %v, want 'Tor process not running'", health.Message())
	}
}

// TestHealthFeatures runs all health-related integration tests with a single Tor instance.
func TestHealthFeatures(t *testing.T) {
	// Use shared global test server
	ts := getGlobalTestServer(t)
	client := ts.Client(t)
	defer client.Close()

	t.Run("ClientCheck", func(t *testing.T) {
		tests := []struct {
			name       string
			client     *Client
			wantStatus HealthStatus
		}{
			{
				name:       "should return healthy status for working client",
				client:     client,
				wantStatus: HealthStatusHealthy,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := context.Background()
				health := tt.client.Check(ctx)

				if health.Status() != tt.wantStatus {
					t.Errorf("Check() status = %v, want %v (message: %s)",
						health.Status(), tt.wantStatus, health.Message())
				}

				if health.Timestamp().IsZero() {
					t.Error("Check() timestamp is zero")
				}

				if health.Latency() <= 0 {
					t.Error("Check() latency is not positive")
				}

				// Test query methods
				if tt.wantStatus == HealthStatusHealthy && !health.IsHealthy() {
					t.Error("IsHealthy() = false, want true")
				}
				if tt.wantStatus == HealthStatusDegraded && !health.IsDegraded() {
					t.Error("IsDegraded() = false, want true")
				}
				if tt.wantStatus == HealthStatusUnhealthy && !health.IsUnhealthy() {
					t.Error("IsUnhealthy() = false, want true")
				}
			})
		}
	})

	t.Run("CheckTorDaemon", func(t *testing.T) {
		tests := []struct {
			name       string
			proc       *TorProcess
			wantStatus HealthStatus
		}{
			{
				name:       "should return healthy status for running daemon",
				proc:       ts.Process,
				wantStatus: HealthStatusHealthy,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := context.Background()
				health := CheckTorDaemon(ctx, tt.proc)

				if health.Status() != tt.wantStatus {
					t.Errorf("CheckTorDaemon() status = %v, want %v (message: %s)",
						health.Status(), tt.wantStatus, health.Message())
				}

				if health.Timestamp().IsZero() {
					t.Error("CheckTorDaemon() timestamp is zero")
				}

				if health.Latency() <= 0 {
					t.Error("CheckTorDaemon() latency is not positive")
				}

				if tt.wantStatus == HealthStatusHealthy && !health.IsHealthy() {
					t.Error("IsHealthy() = false, want true")
				}
			})
		}
	})
}
