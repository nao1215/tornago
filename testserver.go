package tornago

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// testTorSocksAddr is the dedicated SocksPort used for Tornago integration tests.
	testTorSocksAddr = "127.0.0.1:19050"
	// testTorControlAddr is the dedicated ControlPort used for Tornago integration tests.
	testTorControlAddr = "127.0.0.1:19051"
)

// TestServer wraps a TorProcess and Server for integration tests.
type TestServer struct {
	// Process points to the TorProcess launched for tests.
	Process *TorProcess
	// Server exposes the Socks/Control addresses of the launched Tor instance.
	Server Server

	// t holds the testing context for logging/failures.
	t *testing.T
	// clientMu protects lazy client creation and shutdown.
	clientMu sync.Mutex
	// client caches the Client instance connected to this server.
	client *Client
	// controlAuth stores credentials for ControlPort access.
	controlAuth ControlAuth
}

// StartTestServer launches a Tor daemon for tests using a project-local DataDirectory
// and dedicated ports, skipping if tor is unavailable.
func StartTestServer(t *testing.T) *TestServer {
	t.Helper()

	// Use external Tor if configured via env.
	if ctrl := os.Getenv("TORNAGO_TOR_CONTROL"); ctrl != "" {
		return startExternalTestServer(t, ctrl)
	}

	home := os.Getenv("HOME")
	if home == "" {
		t.Fatalf("tornago: HOME environment variable is not set")
	}

	baseDir := filepath.Join(home, ".cache", "tornago-test")
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatalf("tornago: failed to create base tor directory: %v", err)
	}

	dataDir := filepath.Join(baseDir, fmt.Sprintf("test-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("tornago: failed to create tor data directory: %v", err)
	}

	cookiePath := filepath.Join(dataDir, "control_auth_cookie")
	torrcPath := filepath.Join(baseDir, fmt.Sprintf("torrc-%d", time.Now().UnixNano()))
	torrc := fmt.Sprintf(`
SocksPort %s
ControlPort %s
DataDirectory %s
CookieAuthentication 1
CookieAuthFile %s
ClientUseIPv6 0
RunAsDaemon 0
Log notice stdout
`, testTorSocksAddr, testTorControlAddr, dataDir, cookiePath)

	if err := os.WriteFile(torrcPath, []byte(strings.TrimSpace(torrc)+"\n"), 0o600); err != nil {
		t.Fatalf("tornago: failed to write torrc: %v", err)
	}

	bootstrapped := make(chan struct{}, 1)

	launchCfg, err := NewTorLaunchConfig(
		WithTorDataDir(dataDir),
		WithTorSocksAddr(testTorSocksAddr),
		WithTorControlAddr(testTorControlAddr),
		WithTorConfigFile(torrcPath),
	)
	if err != nil {
		t.Fatalf("tornago: failed to build launch config: %v", err)
	}

	process, err := StartTorDaemon(launchCfg)
	if err != nil {
		var te *TornagoError
		if errors.As(err, &te) && te.Kind == ErrTorBinaryNotFound {
			t.Skipf("tornago: skipping because tor binary not found: %v", err)
		}
		t.Fatalf("tornago: failed to start tor daemon: %v", err)
	}

	// Wait for Tor to start before accessing control port
	// Give Tor time to initialize and create the cookie file
	time.Sleep(20 * time.Second)

	// Wait for cookie file
	if err := waitForCookieFile(cookiePath, 30*time.Second); err != nil {
		t.Logf("tornago: skipping integration test because control cookie is unavailable: %v", err)
		t.SkipNow()
	}

	// Obtain control cookie & auth; this function should internally retry
	// until PROTOCOLINFO succeeds and cookie can be read.
	controlAuth, cookiePath, err := ControlAuthFromTor(process.ControlAddr(), 30*time.Second)
	if err != nil {
		t.Logf("tornago: skipping integration test because control cookie could not be read: %v", err)
		t.SkipNow()
	}
	t.Logf("tornago: control cookie path %s", cookiePath)

	serverCfg, err := NewServerConfig(
		WithServerSocksAddr(process.SocksAddr()),
		WithServerControlAddr(process.ControlAddr()),
	)
	if err != nil {
		if stopErr := process.Stop(); stopErr != nil {
			t.Logf("tornago: failed to stop tor after server config error: %v", stopErr)
		}
		t.Fatalf("tornago: failed to build server config: %v", err)
	}

	server, err := NewServer(serverCfg)
	if err != nil {
		if stopErr := process.Stop(); stopErr != nil {
			t.Logf("tornago: failed to stop tor after server init error: %v", stopErr)
		}
		t.Fatalf("tornago: failed to build server: %v", err)
	}

	// Explicitly wait until Tor reports bootstrap 100% via control port.
	// Use a generous timeout since bootstrap can take several minutes depending on network conditions
	if err := waitForTorBootstrap(process.ControlAddr(), controlAuth, 5*time.Minute); err != nil {
		t.Logf("tornago: skipping integration test because tor failed to bootstrap: %v", err)
		t.SkipNow()
	}

	// Also wait for the log-based bootstrap signal
	select {
	case <-bootstrapped:
		t.Log("tornago: bootstrap 100% confirmed via logs")
	case <-time.After(5 * time.Second):
		// This is OK - we already verified via control port
	}

	return &TestServer{
		Process:     process,
		Server:      server,
		t:           t,
		controlAuth: controlAuth,
	}
}

func waitForCookieFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err == nil {
			if info.Size() > 0 {
				return nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for cookie file %s", path)
}

// Client returns a Client configured to use the started Tor instance.
func (ts *TestServer) Client(t *testing.T) *Client {
	t.Helper()
	ts.clientMu.Lock()
	defer ts.clientMu.Unlock()

	if ts.client != nil {
		return ts.client
	}

	cfg, err := NewClientConfig(
		WithClientSocksAddr(ts.Server.SocksAddr()),
		WithClientRequestTimeout(5*time.Minute),
		WithClientDialTimeout(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("tornago: failed to build client config: %v", err)
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("tornago: failed to create client: %v", err)
	}
	ts.client = client
	return client
}

// Close shuts down the client and Tor process launched for tests.
func (ts *TestServer) Close() {
	if ts == nil {
		return
	}
	ts.clientMu.Lock()
	client := ts.client
	ts.client = nil
	ts.clientMu.Unlock()
	if client != nil {
		if err := client.Close(); err != nil {
			if ts.t != nil {
				ts.t.Logf("tornago: failed to close client: %v", err)
			}
		}
	}
	if ts.Process != nil {
		if err := ts.Process.Stop(); err != nil {
			if ts.t != nil {
				// Log instead of Fatal since this is called from defer
				ts.t.Logf("tornago: failed to stop tor process: %v", err)
			}
		}
		ts.Process = nil
	}
}

// ControlAuth returns ControlPort credentials for this TestServer.
func (ts *TestServer) ControlAuth(t *testing.T) ControlAuth {
	t.Helper()
	return ts.controlAuth
}

func waitForTorBootstrap(controlAddr string, auth ControlAuth, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		client, err := NewControlClient(controlAddr, auth, 10*time.Second)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		info, infoErr := client.GetInfo(ctx, "status/bootstrap-phase")
		cancel()
		_ = client.Close()
		if infoErr == nil {
			if progress, ok := parseBootstrapProgress(info); ok {
				if progress == 100 {
					return nil
				}
				lastErr = fmt.Errorf("bootstrap progress %d%%", progress)
			} else {
				lastErr = errors.New("tor not fully bootstrapped")
			}
		} else {
			lastErr = infoErr
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("timed out waiting for tor bootstrap")
	}
	return fmt.Errorf("tor failed to bootstrap: %w", lastErr)
}

func parseBootstrapProgress(info string) (int, bool) {
	idx := strings.LastIndex(info, "PROGRESS=")
	if idx < 0 {
		return 0, false
	}
	start := idx + len("PROGRESS=")
	end := start
	for end < len(info) && info[end] >= '0' && info[end] <= '9' {
		end++
	}
	if start == end {
		return 0, false
	}
	progress, err := strconv.Atoi(info[start:end])
	if err != nil {
		return 0, false
	}
	return progress, true
}

func startExternalTestServer(t *testing.T, controlAddr string) *TestServer {
	t.Helper()
	socksAddr := os.Getenv("TORNAGO_TOR_SOCKS")
	if socksAddr == "" {
		t.Skip("tornago: TORNAGO_TOR_SOCKS not set")
	}
	cookiePath := os.Getenv("TORNAGO_TOR_COOKIE")
	password := os.Getenv("TORNAGO_TOR_PASSWORD")
	if cookiePath != "" && password != "" {
		t.Fatalf("tornago: set either TORNAGO_TOR_COOKIE or TORNAGO_TOR_PASSWORD, not both")
	}

	var controlAuth ControlAuth
	switch {
	case password != "":
		controlAuth = ControlAuthFromPassword(password)
	case cookiePath != "":
		data, err := os.ReadFile(filepath.Clean(cookiePath))
		if err != nil {
			t.Fatalf("tornago: failed to read control cookie %s: %v", cookiePath, err)
		}
		controlAuth = ControlAuthFromCookieBytes(data)
	default:
		t.Fatalf("tornago: TORNAGO_TOR_COOKIE or TORNAGO_TOR_PASSWORD must be set")
	}

	serverCfg, err := NewServerConfig(
		WithServerSocksAddr(socksAddr),
		WithServerControlAddr(controlAddr),
	)
	if err != nil {
		t.Fatalf("tornago: failed to build server config: %v", err)
	}
	server, err := NewServer(serverCfg)
	if err != nil {
		t.Fatalf("tornago: failed to build server: %v", err)
	}

	if err := WaitForControlPort(controlAddr, 30*time.Second); err != nil {
		t.Skipf("tornago: skipping integration test (external tor control port unavailable): %v", err)
	}
	if err := waitForTorBootstrap(controlAddr, controlAuth, 5*time.Minute); err != nil {
		t.Skipf("tornago: skipping integration test (external tor failed to bootstrap): %v", err)
	}

	return &TestServer{
		Server:      server,
		t:           t,
		controlAuth: controlAuth,
	}
}
