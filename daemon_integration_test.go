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

	torrcPath := filepath.Join(tempDir, "torrc")
	torrc := fmt.Sprintf(`
SocksPort %s
ControlPort %s
DataDirectory %s
CookieAuthentication 1
ClientUseIPv6 0
Log notice stdout
`, testTorSocksAddr, testTorControlAddr, dataDir)
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
		WithTorSocksAddr(testTorSocksAddr),
		WithTorControlAddr(testTorControlAddr),
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

	expected := fmt.Sprintf("Read configuration file %q", torrcPath)
	if !strings.Contains(logged, expected) {
		t.Fatalf("tor logs missing config path; want %q got %q", expected, logged)
	}
	if strings.Contains(logged, "/etc/tor/torrc") {
		t.Fatalf("tor logs referenced system torrc; got %q", logged)
	}
}
