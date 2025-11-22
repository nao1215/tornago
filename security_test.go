package tornago

import (
	"context"
	"testing"
	"time"
)

func TestTorConnectionStatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       TorConnectionStatus
		wantContains []string
	}{
		{
			name: "should format using Tor status",
			status: TorConnectionStatus{
				usingTor: true,
				exitIP:   "185.220.101.1",
				message:  "Connection verified",
				latency:  100 * time.Millisecond,
			},
			wantContains: []string{"Using Tor", "185.220.101.1", "100ms"},
		},
		{
			name: "should format not using Tor status",
			status: TorConnectionStatus{
				usingTor: false,
				exitIP:   "192.168.1.1",
				message:  "Not using Tor",
				latency:  50 * time.Millisecond,
			},
			wantContains: []string{"NOT using Tor", "192.168.1.1", "50ms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.status.String()
			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("String() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}

func TestDNSLeakCheckString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		check        DNSLeakCheck
		wantContains []string
	}{
		{
			name: "should format no leak status",
			check: DNSLeakCheck{
				hasLeak:     false,
				resolvedIPs: []string{"1.2.3.4"},
				message:     "No leak",
				latency:     100 * time.Millisecond,
			},
			wantContains: []string{"No DNS leak", "1.2.3.4", "100ms"},
		},
		{
			name: "should format leak detected status",
			check: DNSLeakCheck{
				hasLeak:     true,
				resolvedIPs: []string{"1.2.3.4", "5.6.7.8"},
				message:     "Leak detected",
				latency:     150 * time.Millisecond,
			},
			wantContains: []string{"DNS LEAK DETECTED", "1.2.3.4", "5.6.7.8", "150ms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.check.String()
			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("String() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}

func TestVerifyTorConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ts := StartTestServer(t)
	defer ts.Close()

	client := ts.Client(t)
	defer client.Close()

	t.Run("should verify Tor connection successfully", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		status, err := client.VerifyTorConnection(ctx)
		if err != nil {
			t.Fatalf("VerifyTorConnection() error = %v", err)
		}

		if !status.IsUsingTor() {
			t.Errorf("VerifyTorConnection() IsUsingTor = false, want true (ExitIP: %s, Message: %s)",
				status.ExitIP(), status.Message())
		}

		if status.ExitIP() == "" || status.ExitIP() == unknownIP {
			t.Errorf("VerifyTorConnection() ExitIP = %s, want valid IP", status.ExitIP())
		}

		if status.Latency() <= 0 {
			t.Error("VerifyTorConnection() Latency should be positive")
		}

		t.Logf("Tor connection verified: %s", status)
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.VerifyTorConnection(ctx)
		if err == nil {
			t.Error("VerifyTorConnection() should return error when context is cancelled")
		}
	})
}

func TestCheckDNSLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ts := StartTestServer(t)
	defer ts.Close()

	client := ts.Client(t)
	defer client.Close()

	t.Run("should check DNS leak", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		leakCheck, err := client.CheckDNSLeak(ctx)
		if err != nil {
			t.Fatalf("CheckDNSLeak() error = %v", err)
		}

		if len(leakCheck.ResolvedIPs()) == 0 {
			t.Error("CheckDNSLeak() ResolvedIPs is empty")
		}

		if leakCheck.Latency() <= 0 {
			t.Error("CheckDNSLeak() Latency should be positive")
		}

		t.Logf("DNS leak check: %s", leakCheck)
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.CheckDNSLeak(ctx)
		if err == nil {
			t.Error("CheckDNSLeak() should return error when context is cancelled")
		}
	})
}

// contains is a helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

// findSubstring checks if substr exists in s.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
