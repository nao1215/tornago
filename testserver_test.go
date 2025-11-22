package tornago

import (
	"flag"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// globalTestServer is shared across all integration tests to avoid starting Tor multiple times.
var (
	globalTestServer     *TestServer
	globalTestServerOnce sync.Once
)

// TestMain runs all tests and ensures cleanup of shared resources.
func TestMain(m *testing.M) {
	// Parse flags first so we can check testing.Short()
	flag.Parse()

	// Run all tests
	code := m.Run()

	// Clean up: stop the shared Tor instance if it was started
	if globalTestServer != nil {
		globalTestServer.Close()
	}

	os.Exit(code)
}

// getGlobalTestServer returns the shared test server for integration tests.
// It lazily initializes the server on first call using sync.Once.
func getGlobalTestServer(t *testing.T) *TestServer {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Lazy initialization: start Tor only on first call
	globalTestServerOnce.Do(func() {
		globalTestServer = StartTestServer(t)
	})

	if globalTestServer == nil {
		t.Skip("global test server not available")
	}

	return globalTestServer
}

func TestParseBootstrapProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantProg int
		wantOK   bool
	}{
		{
			name:     "should_parse_100_percent_bootstrap",
			input:    "NOTICE BOOTSTRAP PROGRESS=100 TAG=done SUMMARY=\"Done\"",
			wantProg: 100,
			wantOK:   true,
		},
		{
			name:     "should_parse_partial_bootstrap",
			input:    "NOTICE BOOTSTRAP PROGRESS=50 TAG=loading_descriptors",
			wantProg: 50,
			wantOK:   true,
		},
		{
			name:     "should_parse_zero_progress",
			input:    "NOTICE BOOTSTRAP PROGRESS=0 TAG=starting",
			wantProg: 0,
			wantOK:   true,
		},
		{
			name:     "should_return_false_for_missing_progress",
			input:    "NOTICE BOOTSTRAP TAG=done",
			wantProg: 0,
			wantOK:   false,
		},
		{
			name:     "should_return_false_for_empty_string",
			input:    "",
			wantProg: 0,
			wantOK:   false,
		},
		{
			name:     "should_use_last_progress_when_multiple_exist",
			input:    "PROGRESS=10 then PROGRESS=90",
			wantProg: 90,
			wantOK:   true,
		},
		{
			name:     "should_return_false_for_malformed_progress",
			input:    "PROGRESS=abc",
			wantProg: 0,
			wantOK:   false,
		},
		{
			name:     "should_return_false_for_progress_without_value",
			input:    "PROGRESS=",
			wantProg: 0,
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prog, ok := parseBootstrapProgress(tt.input)
			if prog != tt.wantProg || ok != tt.wantOK {
				t.Errorf("parseBootstrapProgress(%q) = (%d, %v), want (%d, %v)",
					tt.input, prog, ok, tt.wantProg, tt.wantOK)
			}
		})
	}
}

func TestWaitForCookieFile(t *testing.T) {
	t.Parallel()

	t.Run("should_return_immediately_when_file_exists", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cookiePath := filepath.Join(tmpDir, "cookie")

		// Create file with content
		if err := os.WriteFile(cookiePath, []byte("cookie_data"), 0600); err != nil {
			t.Fatalf("failed to create cookie file: %v", err)
		}

		start := time.Now()
		err := waitForCookieFile(cookiePath, 5*time.Second)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if elapsed > 500*time.Millisecond {
			t.Errorf("expected immediate return, took %v", elapsed)
		}
	})

	t.Run("should_wait_for_file_creation", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cookiePath := filepath.Join(tmpDir, "cookie")

		// Create file after delay
		go func() {
			time.Sleep(200 * time.Millisecond)
			if err := os.WriteFile(cookiePath, []byte("cookie_data"), 0600); err != nil {
				t.Logf("failed to create cookie file: %v", err)
			}
		}()

		err := waitForCookieFile(cookiePath, 2*time.Second)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("should_timeout_when_file_not_created", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cookiePath := filepath.Join(tmpDir, "nonexistent")

		err := waitForCookieFile(cookiePath, 100*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error, got nil")
		}
	})

	t.Run("should_wait_for_non_empty_file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cookiePath := filepath.Join(tmpDir, "cookie")

		// Create empty file first
		if err := os.WriteFile(cookiePath, []byte{}, 0600); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		// Write content after delay
		go func() {
			time.Sleep(200 * time.Millisecond)
			if err := os.WriteFile(cookiePath, []byte("cookie_data"), 0600); err != nil {
				t.Logf("failed to write cookie file: %v", err)
			}
		}()

		err := waitForCookieFile(cookiePath, 2*time.Second)
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})
}
