package tornago

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestStartTorDaemonUsesExplicitConfig ensures tor reads the generated torrc.
func TestStartTorDaemonUsesExplicitConfig(t *testing.T) {
	requireIntegration(t)

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("tornago: failed to create tor data directory: %v", err)
	}

	// Resolve dynamic ports before writing to torrc (Tor doesn't support :0 in config files)
	socksAddr, err := resolveAddr("127.0.0.1:0")
	if err != nil {
		t.Fatalf("tornago: failed to resolve socks address: %v", err)
	}
	controlAddr, err := resolveAddr("127.0.0.1:0")
	if err != nil {
		t.Fatalf("tornago: failed to resolve control address: %v", err)
	}

	torrcPath := filepath.Join(tempDir, "torrc")
	torrc := fmt.Sprintf(`
SocksPort %s
ControlPort %s
DataDirectory %s
CookieAuthentication 1
ClientUseIPv6 0
Log notice stdout
`, socksAddr, controlAddr, dataDir)
	if err := os.WriteFile(torrcPath, []byte(strings.TrimSpace(torrc)+"\n"), 0o600); err != nil {
		t.Fatalf("tornago: failed to write torrc: %v", err)
	}

	var (
		mu   sync.Mutex
		logs []string
	)
	logReporter := func(msg string) {
		mu.Lock()
		logs = append(logs, msg)
		mu.Unlock()
	}

	launchCfg, err := NewTorLaunchConfig(
		WithTorDataDir(dataDir),
		WithTorSocksAddr(socksAddr),
		WithTorControlAddr(controlAddr),
		WithTorConfigFile(torrcPath),
		WithTorLogReporter(logReporter),
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
	defer func() {
		if stopErr := process.Stop(); stopErr != nil {
			// Log instead of Fatal since this is called from defer
			t.Logf("tornago: failed to stop tor process: %v", stopErr)
		}
	}()

	mu.Lock()
	logged := strings.Join(logs, "\n")
	mu.Unlock()

	// Check if torrc path appears in logs (without quoting to avoid Windows path escaping issues)
	if !strings.Contains(logged, torrcPath) {
		t.Fatalf("tor logs missing config path %s; got %q", torrcPath, logged)
	}
	if strings.Contains(logged, "/etc/tor/torrc") {
		t.Fatalf("tor logs referenced system torrc; got %q", logged)
	}
}
