package tornago

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("TORNAGO_INTEGRATION") != "1" {
		t.Skip("set TORNAGO_INTEGRATION=1 to run integration tests (requires tor + network access)")
	}
}

func TestClientIntegration(t *testing.T) {
	requireIntegration(t)

	ts := StartTestServer(t)
	defer ts.Close()

	// Create client with ControlAddr configured for hidden service management.
	auth := ts.ControlAuth(t)
	opts := []ClientOption{
		WithClientSocksAddr(ts.Server.SocksAddr()),
		WithClientControlAddr(ts.Server.ControlAddr()),
		WithClientRequestTimeout(5 * time.Minute),
		WithClientDialTimeout(5 * time.Minute),
	}
	if auth.Password() != "" {
		opts = append(opts, WithClientControlPassword(auth.Password()))
	}
	if auth.CookiePath() != "" {
		opts = append(opts, WithClientControlCookie(auth.CookiePath()))
	}
	if len(auth.CookieBytes()) > 0 {
		opts = append(opts, WithClientControlCookieBytes(auth.CookieBytes()))
	}

	cfg, err := NewClientConfig(opts...)
	if err != nil {
		t.Fatalf("NewClientConfig: %v", err)
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	controlClient := client.Control()
	if controlClient == nil {
		t.Fatal("Control() returned nil")
	}

	t.Run("Listen", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		listener, err := client.Listen(ctx, 80, 0)
		if err != nil {
			t.Fatalf("Listen: %v", err)
		}
		defer listener.Close()

		if listener.OnionAddress() == "" {
			t.Error("expected non-empty onion address")
		}
		if listener.VirtualPort() != 80 {
			t.Errorf("VirtualPort() = %d, want 80", listener.VirtualPort())
		}
		if listener.Addr() == nil {
			t.Error("Addr() returned nil")
		}
		if listener.HiddenService() == nil {
			t.Error("HiddenService() returned nil")
		}

		t.Logf("created listener at: %s", listener.OnionAddress())
	})

	t.Run("ListenWithConfig", func(t *testing.T) {
		hsCfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(443, 8443),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		listener, err := client.ListenWithConfig(ctx, hsCfg, 8443)
		if err != nil {
			t.Fatalf("ListenWithConfig: %v", err)
		}
		defer listener.Close()

		if listener.OnionAddress() == "" {
			t.Error("expected non-empty onion address")
		}
		if listener.VirtualPort() != 443 {
			t.Errorf("VirtualPort() = %d, want 443", listener.VirtualPort())
		}

		t.Logf("created listener at: %s", listener.OnionAddress())
	})

	t.Run("HiddenServiceRoundTrip", func(t *testing.T) {
		tt := t
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte("tornago hidden service")); err != nil {
				tt.Logf("failed to write response: %v", err)
			}
		}))
		defer srv.Close()

		port := extractPort(t, srv.URL)
		hsCfg, err := NewHiddenServiceConfig(WithHiddenServicePort(80, port))
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		hs, err := controlClient.CreateHiddenService(ctx, hsCfg)
		if err != nil {
			t.Fatalf("CreateHiddenService failed: %v", err)
		}
		defer func() {
			rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer rmCancel()
			if err := hs.Remove(rmCtx); err != nil {
				t.Logf("hidden service removal error: %v", err)
			}
		}()

		t.Logf("created hidden service: %s", hs.OnionAddress())

		if hs.PrivateKey() == "" {
			t.Error("expected non-empty private key")
		}
		ports := hs.Ports()
		if len(ports) == 0 {
			t.Error("expected non-empty ports map")
		}
		authList := hs.ClientAuth()
		if authList == nil {
			t.Error("expected non-nil client auth slice")
		}

		// Hidden service connections can take time to propagate through the Tor network.
		// Retry with backoff to handle transient failures.
		var resp *http.Response
		var lastErr error
		for attempt := 1; attempt <= 5; attempt++ {
			reqCtx, reqCancel := context.WithTimeout(context.Background(), 90*time.Second)
			req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+hs.OnionAddress(), http.NoBody)
			if err != nil {
				reqCancel()
				t.Fatalf("failed to build onion request: %v", err)
			}
			resp, lastErr = client.HTTP().Do(req)
			reqCancel()
			if lastErr == nil {
				break
			}
			t.Logf("attempt %d failed: %v", attempt, lastErr)
			if attempt < 5 {
				time.Sleep(time.Duration(attempt*10) * time.Second)
			}
		}
		if lastErr != nil {
			t.Fatalf("failed to GET hidden service after retries: %v", lastErr)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected hidden service status: %s", resp.Status)
		}
	})

	t.Run("DialContext", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Dial to a known reachable address via Tor.
		conn, err := client.DialContext(ctx, "tcp", "example.com:80")
		if err != nil {
			t.Fatalf("DialContext: %v", err)
		}
		defer conn.Close()

		if conn.RemoteAddr() == nil {
			t.Error("RemoteAddr() returned nil")
		}
	})

	t.Run("Dialer", func(t *testing.T) {
		dialer := client.Dialer()
		if dialer == nil {
			t.Fatal("Dialer() returned nil")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		conn, err := dialer(ctx, "tcp", "example.com:80")
		if err != nil {
			t.Fatalf("dialer: %v", err)
		}
		defer conn.Close()
	})

	// Helper to create a fresh ControlClient for each test to avoid connection state issues.
	newFreshControl := func(t *testing.T) *ControlClient {
		t.Helper()
		ctrl, err := NewControlClient(ts.Server.ControlAddr(), auth, 30*time.Second)
		if err != nil {
			t.Fatalf("NewControlClient: %v", err)
		}
		return ctrl
	}

	t.Run("ControlClient_GetInfo", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		version, err := ctrl.GetInfo(ctx, "version")
		if err != nil {
			t.Fatalf("GetInfo(version): %v", err)
		}
		if version == "" {
			t.Error("expected non-empty version")
		}
		t.Logf("Tor version: %s", version)
	})

	t.Run("ControlClient_GetConf", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		socksPort, err := ctrl.GetConf(ctx, "SocksPort")
		if err != nil {
			t.Fatalf("GetConf(SocksPort): %v", err)
		}
		if socksPort == "" {
			t.Error("expected non-empty SocksPort")
		}
		t.Logf("SocksPort: %s", socksPort)
	})

	t.Run("ControlClient_NewIdentity", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := ctrl.NewIdentity(ctx)
		if err != nil {
			t.Fatalf("NewIdentity: %v", err)
		}
	})

	t.Run("ControlClient_CircuitStatus", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		circuits, err := ctrl.GetCircuitStatus(ctx)
		if err != nil {
			t.Fatalf("GetCircuitStatus: %v", err)
		}
		t.Logf("circuits: %d", len(circuits))
	})

	t.Run("ControlClient_StreamStatus", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		streams, err := ctrl.GetStreamStatus(ctx)
		if err != nil {
			t.Fatalf("GetStreamStatus: %v", err)
		}
		t.Logf("streams: %d", len(streams))
	})
}

func extractPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed to parse url %s: %v", rawURL, err)
	}
	host := u.Host
	if strings.Contains(host, ":") {
		_, portStr, err := net.SplitHostPort(host)
		if err != nil {
			t.Fatalf("failed to split host port: %v", err)
		}
		p, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatalf("failed to parse port %s: %v", portStr, err)
		}
		return p
	}
	t.Fatalf("url missing port: %s", rawURL)
	return 0
}
