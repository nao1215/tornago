package tornago

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTorLaunchConfig(t *testing.T) {
	t.Run("should apply default values when no options provided", func(t *testing.T) {
		cfg, err := NewTorLaunchConfig()
		if err != nil {
			t.Fatalf("NewTorLaunchConfig returned error: %v", err)
		}

		if cfg.TorBinary() == "" {
			t.Errorf("TorBinary is empty")
		}
		if cfg.SocksAddr() == "" {
			t.Errorf("SocksAddr is empty")
		}
		if cfg.ControlAddr() == "" {
			t.Errorf("ControlAddr is empty")
		}
		if cfg.StartupTimeout() <= 0 {
			t.Errorf("StartupTimeout must be positive")
		}
	})

	t.Run("should reject negative startup timeout", func(t *testing.T) {
		_, err := NewTorLaunchConfig(WithTorStartupTimeout(-1 * time.Second))
		if err == nil {
			t.Fatalf("expected error when StartupTimeout <= 0")
		}
	})

	t.Run("should accept custom data directory", func(t *testing.T) {
		tDir := t.TempDir()
		custom := filepath.Join(tDir, "tor-data")

		cfg, err := NewTorLaunchConfig(WithTorDataDir(custom))
		if err != nil {
			t.Fatalf("NewTorLaunchConfig returned error: %v", err)
		}
		if cfg.DataDir() != filepath.Clean(custom) {
			t.Fatalf("DataDir mismatch: want %s got %s", filepath.Clean(custom), cfg.DataDir())
		}
	})

	t.Run("should accept log reporter callback", func(t *testing.T) {
		reporter := func(string) {}
		cfg, err := NewTorLaunchConfig(WithTorLogReporter(reporter))
		if err != nil {
			t.Fatalf("NewTorLaunchConfig returned error: %v", err)
		}
		if cfg.LogReporter() == nil {
			t.Fatalf("LogReporter should be set")
		}
	})

	t.Run("should accept custom torrc file path", func(t *testing.T) {
		torrcPath := "/tmp/custom-torrc"
		cfg, err := NewTorLaunchConfig(WithTorConfigFile(torrcPath))
		if err != nil {
			t.Fatalf("NewTorLaunchConfig returned error: %v", err)
		}
		if cfg.TorConfigFile() != torrcPath {
			t.Errorf("TorConfigFile mismatch: want %s got %s", torrcPath, cfg.TorConfigFile())
		}
	})

	t.Run("should accept extra command line arguments", func(t *testing.T) {
		extraArgs := []string{"--DisableNetwork", "1"}
		cfg, err := NewTorLaunchConfig(WithTorExtraArgs(extraArgs...))
		if err != nil {
			t.Fatalf("NewTorLaunchConfig returned error: %v", err)
		}
		args := cfg.ExtraArgs()
		if len(args) != 2 || args[0] != "--DisableNetwork" || args[1] != "1" {
			t.Errorf("ExtraArgs mismatch: got %v", args)
		}
	})
}

func TestNewServerConfig(t *testing.T) {
	t.Run("should apply default socks and control addresses", func(t *testing.T) {
		cfg, err := NewServerConfig()
		if err != nil {
			t.Fatalf("NewServerConfig returned error: %v", err)
		}
		if cfg.SocksAddr() == "" || cfg.ControlAddr() == "" {
			t.Fatalf("server config defaults not applied: %+v", cfg)
		}
	})

	t.Run("should accept custom socks and control addresses", func(t *testing.T) {
		custom, err := NewServerConfig(
			WithServerSocksAddr("127.0.0.1:10000"),
			WithServerControlAddr("127.0.0.1:10001"),
		)
		if err != nil {
			t.Fatalf("custom server config failed: %v", err)
		}
		if custom.SocksAddr() != "127.0.0.1:10000" {
			t.Errorf("custom SocksAddr not applied: got %s", custom.SocksAddr())
		}
		if custom.ControlAddr() != "127.0.0.1:10001" {
			t.Errorf("custom ControlAddr not applied: got %s", custom.ControlAddr())
		}
	})
}

func TestNewClientConfig(t *testing.T) {
	t.Run("should apply default timeout and retry settings", func(t *testing.T) {
		cfg, err := NewClientConfig(WithClientSocksAddr("127.0.0.1:9050"))
		if err != nil {
			t.Fatalf("NewClientConfig returned error: %v", err)
		}
		if cfg.DialTimeout() <= 0 {
			t.Errorf("DialTimeout should be positive: got %v", cfg.DialTimeout())
		}
		if cfg.RequestTimeout() <= 0 {
			t.Errorf("RequestTimeout should be positive: got %v", cfg.RequestTimeout())
		}
		if cfg.RetryDelay() <= 0 {
			t.Errorf("RetryDelay should be positive: got %v", cfg.RetryDelay())
		}
		if cfg.RetryMaxDelay() < cfg.RetryDelay() {
			t.Errorf("RetryMaxDelay should be >= RetryDelay: delay=%v max=%v",
				cfg.RetryDelay(), cfg.RetryMaxDelay())
		}
		if cfg.RetryAttempts() == 0 {
			t.Errorf("RetryAttempts should default > 0")
		}
		if cfg.RetryOnError() == nil {
			t.Errorf("RetryOnError must not be nil")
		}
	})

	t.Run("should reject negative retry delay", func(t *testing.T) {
		_, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithRetryDelay(-1*time.Second),
		)
		if err == nil {
			t.Fatalf("expected error for negative retry delay")
		}
	})

	t.Run("should apply default socks address when not provided", func(t *testing.T) {
		cfg, err := NewClientConfig()
		if err != nil {
			t.Fatalf("NewClientConfig returned error: %v", err)
		}
		if cfg.SocksAddr() == "" {
			t.Fatalf("expected default SocksAddr to be set")
		}
	})

	t.Run("should accept control port configuration", func(t *testing.T) {
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithClientControlAddr("127.0.0.1:9051"),
			WithClientControlPassword("test-password"),
		)
		if err != nil {
			t.Fatalf("NewClientConfig returned error: %v", err)
		}
		if cfg.ControlAddr() != "127.0.0.1:9051" {
			t.Errorf("ControlAddr mismatch: got %s", cfg.ControlAddr())
		}
		if cfg.ControlAuth().Password() != "test-password" {
			t.Errorf("ControlAuth password not set correctly")
		}
	})
}

func TestControlAuth(t *testing.T) {
	t.Run("should store and return cookie bytes defensively", func(t *testing.T) {
		cookie := []byte{0x01, 0x02, 0x03}
		auth := ControlAuthFromCookieBytes(cookie)
		returned := auth.CookieBytes()
		if len(returned) != len(cookie) {
			t.Fatalf("CookieBytes length mismatch: want %d got %d", len(cookie), len(returned))
		}
		if returned[0] != 0x01 {
			t.Fatalf("CookieBytes content mismatch: got %v", returned)
		}
		// Modify returned slice to ensure defensive copy
		returned[0] = 0xFF
		if auth.CookieBytes()[0] != 0x01 {
			t.Fatalf("CookieBytes should be defensive copy")
		}
	})

	t.Run("should create auth from password", func(t *testing.T) {
		auth := ControlAuthFromPassword("test-password")
		if auth.Password() != "test-password" {
			t.Errorf("Password mismatch: got %s", auth.Password())
		}
		if len(auth.CookieBytes()) != 0 {
			t.Errorf("CookieBytes should be empty when using password")
		}
	})

	t.Run("should create auth from cookie path", func(t *testing.T) {
		auth := ControlAuthFromCookie("/path/to/cookie")
		if auth.CookiePath() != "/path/to/cookie" {
			t.Errorf("CookiePath mismatch: got %s", auth.CookiePath())
		}
	})
}

func TestWithTorBinary(t *testing.T) {
	t.Run("should set custom tor binary path", func(t *testing.T) {
		cfg, err := NewTorLaunchConfig(
			WithTorBinary("/custom/path/to/tor"),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}
		if cfg.torBinary != "/custom/path/to/tor" {
			t.Errorf("expected torBinary '/custom/path/to/tor', got %s", cfg.torBinary)
		}
	})
}

func TestWithClientControlCookie(t *testing.T) {
	t.Run("should set control cookie path", func(t *testing.T) {
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithClientControlAddr("127.0.0.1:9051"),
			WithClientControlCookie("/path/to/cookie"),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}
		if cfg.ControlAddr() == "" {
			t.Error("ControlAddr should be set")
		}
	})
}

func TestWithRetryMaxDelay(t *testing.T) {
	t.Run("should set maximum retry delay", func(t *testing.T) {
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithRetryMaxDelay(10*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}
		if cfg.retryMaxDelay != 10*time.Second {
			t.Errorf("expected retryMaxDelay 10s, got %v", cfg.retryMaxDelay)
		}
	})
}

func TestWithRetryOnError(t *testing.T) {
	t.Run("should set custom retry error function", func(t *testing.T) {
		retryFunc := func(err error) bool {
			return errors.Is(err, errors.New("specific error"))
		}
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithRetryOnError(retryFunc),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}
		if cfg.retryOnError == nil {
			t.Error("retryOnError should be set")
		}
	})
}

func TestValidateClientConfig(t *testing.T) {
	t.Run("should reject empty SOCKS address", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr: "",
		}
		if err := validateClientConfig(cfg); err == nil {
			t.Error("expected error for empty SOCKS address")
		}
	})

	t.Run("should reject negative dial timeout", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr:   "127.0.0.1:9050",
			dialTimeout: -1 * time.Second,
		}
		if err := validateClientConfig(cfg); err == nil {
			t.Error("expected error for negative dial timeout")
		}
	})

	t.Run("should accept valid configuration", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr:      "127.0.0.1:9050",
			dialTimeout:    30 * time.Second,
			requestTimeout: 60 * time.Second,
			retryDelay:     1 * time.Second,
			retryMaxDelay:  10 * time.Second,
			retryOnError:   defaultRetryOnError,
		}
		if err := validateClientConfig(cfg); err != nil {
			t.Errorf("unexpected error for valid config: %v", err)
		}
	})
}

func TestValidateTorLaunchConfig(t *testing.T) {
	t.Run("should reject empty SOCKS address", func(t *testing.T) {
		cfg := TorLaunchConfig{
			socksAddr: "",
		}
		if err := validateTorLaunchConfig(cfg); err == nil {
			t.Error("expected error for empty SOCKS address")
		}
	})

	t.Run("should reject empty control address", func(t *testing.T) {
		cfg := TorLaunchConfig{
			socksAddr:   "127.0.0.1:9050",
			controlAddr: "",
		}
		if err := validateTorLaunchConfig(cfg); err == nil {
			t.Error("expected error for empty control address")
		}
	})

	t.Run("should accept valid configuration", func(t *testing.T) {
		cfg := TorLaunchConfig{
			torBinary:      "tor",
			socksAddr:      "127.0.0.1:9050",
			controlAddr:    "127.0.0.1:9051",
			startupTimeout: 60 * time.Second,
		}
		if err := validateTorLaunchConfig(cfg); err != nil {
			t.Errorf("unexpected error for valid config: %v", err)
		}
	})

	t.Run("should pass validation with all required fields", func(t *testing.T) {
		cfg := TorLaunchConfig{
			torBinary:      "/usr/bin/tor",
			socksAddr:      "127.0.0.1:9050",
			controlAddr:    "127.0.0.1:9051",
			startupTimeout: 30 * time.Second,
		}
		err := validateTorLaunchConfig(cfg)
		if err != nil {
			t.Errorf("expected validation to pass: %v", err)
		}
	})

	t.Run("should fail validation with empty torBinary", func(t *testing.T) {
		cfg := TorLaunchConfig{
			torBinary:      "",
			socksAddr:      "127.0.0.1:9050",
			controlAddr:    "127.0.0.1:9051",
			startupTimeout: 30 * time.Second,
		}
		err := validateTorLaunchConfig(cfg)
		if err == nil {
			t.Error("expected validation to fail with empty torBinary")
		}
	})

	t.Run("should fail validation with zero startupTimeout", func(t *testing.T) {
		cfg := TorLaunchConfig{
			torBinary:      "/usr/bin/tor",
			socksAddr:      "127.0.0.1:9050",
			controlAddr:    "127.0.0.1:9051",
			startupTimeout: 0,
		}
		err := validateTorLaunchConfig(cfg)
		if err == nil {
			t.Error("expected validation to fail with zero startupTimeout")
		}
	})
}

func TestValidateServerConfig(t *testing.T) {
	t.Run("should reject empty SOCKS address", func(t *testing.T) {
		cfg := ServerConfig{
			socksAddr: "",
		}
		if err := validateServerConfig(cfg); err == nil {
			t.Error("expected error for empty SOCKS address")
		}
	})

	t.Run("should reject empty control address", func(t *testing.T) {
		cfg := ServerConfig{
			socksAddr:   "127.0.0.1:9050",
			controlAddr: "",
		}
		if err := validateServerConfig(cfg); err == nil {
			t.Error("expected error for empty control address")
		}
	})

	t.Run("should accept valid configuration", func(t *testing.T) {
		cfg := ServerConfig{
			socksAddr:   "127.0.0.1:9050",
			controlAddr: "127.0.0.1:9051",
		}
		if err := validateServerConfig(cfg); err != nil {
			t.Errorf("unexpected error for valid config: %v", err)
		}
	})
}

func TestClientConfigValidationEdgeCases(t *testing.T) {
	t.Run("should reject when retryMaxDelay less than retryDelay", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr:      "127.0.0.1:9050",
			dialTimeout:    30 * time.Second,
			requestTimeout: 60 * time.Second,
			retryDelay:     10 * time.Second,
			retryMaxDelay:  5 * time.Second, // Less than retryDelay
			retryOnError:   defaultRetryOnError,
		}
		if err := validateClientConfig(cfg); err == nil {
			t.Error("expected error when retryMaxDelay < retryDelay")
		}
	})

	t.Run("should reject when retryOnError is nil", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr:      "127.0.0.1:9050",
			dialTimeout:    30 * time.Second,
			requestTimeout: 60 * time.Second,
			retryDelay:     1 * time.Second,
			retryMaxDelay:  10 * time.Second,
			retryOnError:   nil, // nil function
		}
		if err := validateClientConfig(cfg); err == nil {
			t.Error("expected error when retryOnError is nil")
		}
	})

	t.Run("should reject negative requestTimeout", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr:      "127.0.0.1:9050",
			dialTimeout:    30 * time.Second,
			requestTimeout: -1 * time.Second,
			retryDelay:     1 * time.Second,
			retryMaxDelay:  10 * time.Second,
			retryOnError:   defaultRetryOnError,
		}
		if err := validateClientConfig(cfg); err == nil {
			t.Error("expected error for negative requestTimeout")
		}
	})

	t.Run("should reject negative retryDelay", func(t *testing.T) {
		cfg := ClientConfig{
			socksAddr:      "127.0.0.1:9050",
			dialTimeout:    30 * time.Second,
			requestTimeout: 60 * time.Second,
			retryDelay:     -1 * time.Second,
			retryMaxDelay:  10 * time.Second,
			retryOnError:   defaultRetryOnError,
		}
		if err := validateClientConfig(cfg); err == nil {
			t.Error("expected error for negative retryDelay")
		}
	})
}

func TestTorLaunchConfigValidationEdgeCases(t *testing.T) {
	t.Run("should reject negative startupTimeout", func(t *testing.T) {
		cfg := TorLaunchConfig{
			torBinary:      "tor",
			socksAddr:      "127.0.0.1:9050",
			controlAddr:    "127.0.0.1:9051",
			startupTimeout: -1 * time.Second,
		}
		if err := validateTorLaunchConfig(cfg); err == nil {
			t.Error("expected error for negative startupTimeout")
		}
	})

	t.Run("should reject zero startupTimeout", func(t *testing.T) {
		cfg := TorLaunchConfig{
			torBinary:      "tor",
			socksAddr:      "127.0.0.1:9050",
			controlAddr:    "127.0.0.1:9051",
			startupTimeout: 0,
		}
		if err := validateTorLaunchConfig(cfg); err == nil {
			t.Error("expected error for zero startupTimeout")
		}
	})
}

func TestNormalizeServerConfig(t *testing.T) {
	t.Run("should apply defaults and validate", func(t *testing.T) {
		cfg := ServerConfig{
			socksAddr:   "127.0.0.1:9050",
			controlAddr: "127.0.0.1:9051",
		}
		normalized, err := normalizeServerConfig(cfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if normalized.socksAddr != "127.0.0.1:9050" {
			t.Errorf("expected socksAddr to be preserved, got %s", normalized.socksAddr)
		}
	})
	t.Run("should apply defaults for empty config", func(t *testing.T) {
		cfg := ServerConfig{}
		normalized, err := normalizeServerConfig(cfg)
		if err != nil {
			t.Fatalf("normalizeServerConfig failed: %v", err)
		}
		if normalized.SocksAddr() == "" {
			t.Error("expected non-empty socks address after normalization")
		}
		if normalized.ControlAddr() == "" {
			t.Error("expected non-empty control address after normalization")
		}
	})

	t.Run("should preserve custom values", func(t *testing.T) {
		cfg := ServerConfig{
			socksAddr:   "127.0.0.1:9999",
			controlAddr: "127.0.0.1:9998",
		}
		normalized, err := normalizeServerConfig(cfg)
		if err != nil {
			t.Fatalf("normalizeServerConfig failed: %v", err)
		}
		if normalized.SocksAddr() != "127.0.0.1:9999" {
			t.Errorf("expected socks address to be preserved: got %s", normalized.SocksAddr())
		}
		if normalized.ControlAddr() != "127.0.0.1:9998" {
			t.Errorf("expected control address to be preserved: got %s", normalized.ControlAddr())
		}
	})
}

func TestNewClientWithInvalidConfig(t *testing.T) {
	t.Run("should reject negative dial timeout", func(t *testing.T) {
		_, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithClientDialTimeout(-1*time.Second),
		)
		if err == nil {
			t.Error("expected error for negative dial timeout")
		}
	})

	t.Run("should reject negative request timeout", func(t *testing.T) {
		_, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithClientRequestTimeout(-1*time.Second),
		)
		if err == nil {
			t.Error("expected error for negative request timeout")
		}
	})
}

func TestNewServerWithValidation(t *testing.T) {
	t.Run("should accept valid config", func(t *testing.T) {
		cfg, err := NewServerConfig(
			WithServerSocksAddr("127.0.0.1:9050"),
			WithServerControlAddr("127.0.0.1:9051"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		server, err := NewServer(cfg)
		if err != nil {
			t.Fatalf("unexpected error creating server: %v", err)
		}

		if server.SocksAddr() != "127.0.0.1:9050" {
			t.Errorf("expected SocksAddr 127.0.0.1:9050, got %s", server.SocksAddr())
		}
	})
}

func TestNewTorLaunchConfigValidation(t *testing.T) {
	t.Run("should reject negative startup timeout", func(t *testing.T) {
		_, err := NewTorLaunchConfig(
			WithTorStartupTimeout(-1 * time.Second),
		)
		if err == nil {
			t.Error("expected error for negative startup timeout")
		}
	})

	t.Run("should accept valid config with all options", func(t *testing.T) {
		cfg, err := NewTorLaunchConfig(
			WithTorSocksAddr("127.0.0.1:9050"),
			WithTorControlAddr("127.0.0.1:9051"),
			WithTorDataDir("/tmp/tor-data"),
			WithTorBinary("/usr/bin/tor"),
			WithTorStartupTimeout(2*time.Minute),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.SocksAddr() != "127.0.0.1:9050" {
			t.Errorf("expected SocksAddr 127.0.0.1:9050, got %s", cfg.SocksAddr())
		}

		if cfg.ControlAddr() != "127.0.0.1:9051" {
			t.Errorf("expected ControlAddr 127.0.0.1:9051, got %s", cfg.ControlAddr())
		}
	})
}
