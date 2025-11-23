package tornago

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTorProcessAccessors(t *testing.T) {
	t.Run("should return correct PID", func(t *testing.T) {
		p := &TorProcess{pid: 12345}
		if p.PID() != 12345 {
			t.Errorf("expected PID 12345, got %d", p.PID())
		}
	})

	t.Run("should return correct SocksAddr", func(t *testing.T) {
		p := &TorProcess{socksAddr: "127.0.0.1:9050"}
		if p.SocksAddr() != "127.0.0.1:9050" {
			t.Errorf("expected SocksAddr 127.0.0.1:9050, got %s", p.SocksAddr())
		}
	})

	t.Run("should return correct ControlAddr", func(t *testing.T) {
		p := &TorProcess{controlAddr: "127.0.0.1:9051"}
		if p.ControlAddr() != "127.0.0.1:9051" {
			t.Errorf("expected ControlAddr 127.0.0.1:9051, got %s", p.ControlAddr())
		}
	})

	t.Run("should return correct DataDir", func(t *testing.T) {
		expectedDir := filepath.Join(os.TempDir(), "tor")
		p := &TorProcess{dataDir: expectedDir}
		if p.DataDir() != expectedDir {
			t.Errorf("expected DataDir %s, got %s", expectedDir, p.DataDir())
		}
	})
}

func TestResolveAddr(t *testing.T) {
	t.Run("should resolve :0 to random port", func(t *testing.T) {
		addr, err := resolveAddr(":0")
		if err != nil {
			t.Fatalf("resolveAddr failed: %v", err)
		}
		if addr == ":0" {
			t.Error("resolveAddr should return concrete port, not :0")
		}
		if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "[::1]:") {
			t.Errorf("unexpected address format: %s", addr)
		}
	})

	t.Run("should keep explicit address unchanged", func(t *testing.T) {
		addr, err := resolveAddr("192.168.1.1:9050")
		if err != nil {
			t.Fatalf("resolveAddr failed: %v", err)
		}
		if addr != "192.168.1.1:9050" {
			t.Errorf("expected 192.168.1.1:9050, got %s", addr)
		}
	})

	t.Run("should reject invalid address format", func(t *testing.T) {
		_, err := resolveAddr("invalid")
		if err == nil {
			t.Error("resolveAddr should fail for invalid address")
		}
	})
}

func TestTeeWriter(t *testing.T) {
	t.Run("should write to buffer and call reporter for complete lines", func(t *testing.T) {
		var reported []string
		reporter := func(msg string) {
			reported = append(reported, msg)
		}

		var buf bytes.Buffer
		writer := &teeWriter{
			buf:      &buf,
			reporter: reporter,
		}

		// Write first line
		n, err := writer.Write([]byte("first line\n"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != 11 {
			t.Errorf("expected to write 11 bytes, got %d", n)
		}

		// Check buffer
		if buf.String() != "first line\n" {
			t.Errorf("expected buffer 'first line\\n', got %q", buf.String())
		}

		// Check reporter was called
		if len(reported) != 1 {
			t.Fatalf("expected 1 reported line, got %d", len(reported))
		}
		if reported[0] != "first line" {
			t.Errorf("expected reported 'first line', got %q", reported[0])
		}
	})

	t.Run("should buffer partial lines", func(t *testing.T) {
		var reported []string
		reporter := func(msg string) {
			reported = append(reported, msg)
		}

		var buf bytes.Buffer
		writer := &teeWriter{
			buf:      &buf,
			reporter: reporter,
		}

		// Write partial line
		_, _ = writer.Write([]byte("partial ")) //nolint:errcheck
		_, _ = writer.Write([]byte("line"))     //nolint:errcheck

		// No complete line, so reporter should not be called
		if len(reported) != 0 {
			t.Errorf("reporter should not be called for partial lines, but got %d calls", len(reported))
		}

		// Complete the line
		_, _ = writer.Write([]byte("\n")) //nolint:errcheck

		if len(reported) != 1 {
			t.Fatalf("expected 1 reported line after newline, got %d", len(reported))
		}
		if reported[0] != "partial line" {
			t.Errorf("expected 'partial line', got %q", reported[0])
		}
	})

	t.Run("should handle multiple lines in single write", func(t *testing.T) {
		var reported []string
		reporter := func(msg string) {
			reported = append(reported, msg)
		}

		var buf bytes.Buffer
		writer := &teeWriter{
			buf:      &buf,
			reporter: reporter,
		}

		_, _ = writer.Write([]byte("line1\nline2\nline3\n")) //nolint:errcheck

		if len(reported) != 3 {
			t.Fatalf("expected 3 reported lines, got %d", len(reported))
		}
		if reported[0] != "line1" || reported[1] != "line2" || reported[2] != "line3" {
			t.Errorf("unexpected reported lines: %v", reported)
		}
	})

	t.Run("should work without reporter", func(t *testing.T) {
		var buf bytes.Buffer
		writer := &teeWriter{
			buf:      &buf,
			reporter: nil, // No reporter
		}

		n, err := writer.Write([]byte("test\n"))
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != 5 {
			t.Errorf("expected to write 5 bytes, got %d", n)
		}

		if buf.String() != "test\n" {
			t.Errorf("expected buffer 'test\\n', got %q", buf.String())
		}
	})
}

// TestTorProcessCrashRecovery tests recovery from Tor process termination.
// This test verifies that clients can detect when the Tor daemon stops unexpectedly.
func TestTorProcessCrashRecovery(t *testing.T) {
	requireIntegration(t)

	// Skip on OpenBSD due to system torrc User option causing permission errors
	// when launching Tor as non-root user in GitHub Actions environment
	if runtime.GOOS == "openbsd" {
		t.Skip("skipping on OpenBSD: system torrc User option conflicts with non-root execution")
	}

	t.Run("client detects when Tor process stops", func(t *testing.T) {
		// Launch a dedicated Tor instance for this test
		launchCfg, err := NewTorLaunchConfig(
			WithTorSocksAddr(":0"),
			WithTorControlAddr(":0"),
			WithTorStartupTimeout(60*time.Second),
		)
		if err != nil {
			t.Fatalf("NewTorLaunchConfig: %v", err)
		}

		torProc, err := StartTorDaemon(launchCfg)
		if err != nil {
			t.Fatalf("StartTorDaemon: %v", err)
		}
		defer torProc.Stop() // Ensure cleanup even if test fails early

		// Create client with control port access to check bootstrap status
		auth, _, err := ControlAuthFromTor(torProc.ControlAddr(), 5*time.Second)
		if err != nil {
			t.Fatalf("ControlAuthFromTor: %v", err)
		}

		opts := []ClientOption{
			WithClientSocksAddr(torProc.SocksAddr()),
			WithClientControlAddr(torProc.ControlAddr()),
			WithClientDialTimeout(10 * time.Second),
			WithClientRequestTimeout(30 * time.Second),
		}
		if auth.Password() != "" {
			opts = append(opts, WithClientControlPassword(auth.Password()))
		} else if auth.CookiePath() != "" {
			opts = append(opts, WithClientControlCookie(auth.CookiePath()))
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

		// Wait for Tor to be fully bootstrapped by checking status
		// This is more reliable than time.Sleep
		if err := waitForTorBootstrap(torProc.ControlAddr(), auth, 60*time.Second); err != nil {
			t.Fatalf("waitForTorBootstrap: %v", err)
		}

		// Verify connection works
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://check.torproject.org/api/ip", http.NoBody)
		if err != nil {
			t.Fatalf("NewRequestWithContext: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("initial request failed: %v", err)
		}
		_ = resp.Body.Close()

		// Terminate Tor process
		// Note: Stop() may return "signal: killed" which is expected
		// We call Stop explicitly here for the test, defer will handle cleanup on early failures
		if err := torProc.Stop(); err != nil {
			t.Logf("Stop returned error (expected): %v", err)
		}

		// Subsequent requests should fail immediately (no sleep needed)
		req2, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", http.NoBody)
		if err != nil {
			t.Fatalf("NewRequestWithContext: %v", err)
		}

		resp2, err := client.Do(req2)
		if err == nil {
			_ = resp2.Body.Close()
			t.Error("expected error after Tor process termination, got nil")
		}
	})
}

// TestTorStartupTimeout tests Tor daemon startup timeout behavior.
// This test is quick because we use a very short timeout.
func TestTorStartupTimeout(t *testing.T) {
	requireIntegration(t)

	t.Run("startup timeout is enforced", func(t *testing.T) {
		// Use impossibly short timeout to trigger timeout error
		launchCfg, err := NewTorLaunchConfig(
			WithTorSocksAddr(":0"),
			WithTorControlAddr(":0"),
			WithTorStartupTimeout(1*time.Millisecond), // Too short
		)
		if err != nil {
			t.Fatalf("NewTorLaunchConfig: %v", err)
		}

		start := time.Now()
		_, err = StartTorDaemon(launchCfg)
		elapsed := time.Since(start)

		if err == nil {
			t.Error("expected timeout error, got nil")
		}

		// Should fail quickly (within 1 second)
		if elapsed > 1*time.Second {
			t.Errorf("timeout took too long: %v (expected < 1s)", elapsed)
		}
	})
}
