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

func TestHiddenServiceRoundTrip(t *testing.T) {
	requireIntegration(t)

	ts := StartTestServer(t)
	defer ts.Close()

	tt := t
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("tornago hidden service")); err != nil {
			tt.Logf("failed to write response: %v", err)
		}
	}))
	defer srv.Close()

	port := extractPort(t, srv.URL)
	controlClient, err := NewControlClient(ts.Server.ControlAddr(), ts.ControlAuth(t), 30*time.Second)
	if err != nil {
		t.Fatalf("failed to create control client: %v", err)
	}
	defer controlClient.Close()

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
	t.Logf("created hidden service: %s", hs.OnionAddress())

	// Test accessor methods
	if hs.PrivateKey() == "" {
		t.Error("expected non-empty private key")
	}
	ports := hs.Ports()
	if len(ports) == 0 {
		t.Error("expected non-empty ports map")
	}
	auth := hs.ClientAuth()
	if auth == nil {
		t.Error("expected non-nil client auth slice")
	}

	defer func() {
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer rmCancel()
		if err := hs.Remove(rmCtx); err != nil {
			t.Logf("hidden service removal error: %v", err)
		}
	}()

	client := ts.Client(t)
	// Hidden service connections can take significant time to establish circuits,
	// especially on first connection (directory propagation, circuit building, etc.)
	reqCtx, reqCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer reqCancel()
	t.Logf("requesting hidden service at: %s", hs.OnionAddress())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+hs.OnionAddress(), http.NoBody)
	if err != nil {
		t.Fatalf("failed to build onion request: %v", err)
	}
	t.Logf("sending HTTP request...")
	resp, err := client.HTTP().Do(req)
	if err != nil {
		t.Fatalf("failed to GET hidden service: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected hidden service status: %s", resp.Status)
	}
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
