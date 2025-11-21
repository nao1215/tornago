package tornago

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewHiddenServiceConfig(t *testing.T) {
	t.Run("should reject configuration with no ports specified", func(t *testing.T) {
		_, err := NewHiddenServiceConfig()
		if err == nil {
			t.Fatalf("expected error when no ports configured")
		}
	})

	t.Run("should reject configuration with invalid negative virtual port", func(t *testing.T) {
		_, err := NewHiddenServiceConfig(
			WithHiddenServicePort(-1, 8080),
		)
		if err == nil {
			t.Fatalf("expected error when virtual port is negative")
		}
	})

	t.Run("should reject configuration with virtual port exceeding 65535", func(t *testing.T) {
		_, err := NewHiddenServiceConfig(
			WithHiddenServicePort(70000, 8080),
		)
		if err == nil {
			t.Fatalf("expected error when virtual port exceeds 65535")
		}
	})

	t.Run("should reject configuration with invalid negative target port", func(t *testing.T) {
		_, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, -1),
		)
		if err == nil {
			t.Fatalf("expected error when target port is negative")
		}
	})

	t.Run("should reject configuration with target port exceeding 65535", func(t *testing.T) {
		_, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 70000),
		)
		if err == nil {
			t.Fatalf("expected error when target port exceeds 65535")
		}
	})

	t.Run("should apply default ED25519-V3 key type when not specified", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		if cfg.KeyType() != "ED25519-V3" {
			t.Fatalf("expected KeyType to default to ED25519-V3, got %s", cfg.KeyType())
		}
	})

	t.Run("should correctly configure single port mapping", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		ports := cfg.Ports()
		if len(ports) != 1 {
			t.Fatalf("expected 1 port mapping, got %d", len(ports))
		}
		if ports[80] != 8080 {
			t.Fatalf("expected port 80 -> 8080, got %d -> %d", 80, ports[80])
		}
	})

	t.Run("should correctly configure multiple port mappings", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServicePort(443, 8443),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		ports := cfg.Ports()
		if len(ports) != 2 {
			t.Fatalf("expected 2 port mappings, got %d", len(ports))
		}
		if ports[80] != 8080 || ports[443] != 8443 {
			t.Fatalf("port mappings incorrect: %+v", ports)
		}
	})

	t.Run("should accept custom key type", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceKeyType("RSA1024"),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		if cfg.KeyType() != "RSA1024" {
			t.Fatalf("expected KeyType RSA1024, got %s", cfg.KeyType())
		}
	})

	t.Run("should store provided private key for reuse", func(t *testing.T) {
		testKey := "ED25519-V3:ABCD1234"
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServicePrivateKey(testKey),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		if cfg.PrivateKey() != testKey {
			t.Fatalf("expected PrivateKey %s, got %s", testKey, cfg.PrivateKey())
		}
	})

	t.Run("should configure port mappings using WithHiddenServicePorts", func(t *testing.T) {
		ports := map[int]int{
			80:  8080,
			443: 8443,
			22:  2222,
		}
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePorts(ports),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		cfgPorts := cfg.Ports()
		if len(cfgPorts) != 3 {
			t.Fatalf("expected 3 port mappings, got %d", len(cfgPorts))
		}
		for virt, tgt := range ports {
			if cfgPorts[virt] != tgt {
				t.Fatalf("expected port %d -> %d, got %d -> %d", virt, tgt, virt, cfgPorts[virt])
			}
		}
	})
}

func TestNewHiddenServiceAuth(t *testing.T) {
	t.Run("should create client auth entry with name and key", func(t *testing.T) {
		auth := NewHiddenServiceAuth("client1", "test-key-base32")
		if auth.ClientName() != "client1" {
			t.Fatalf("expected ClientName client1, got %s", auth.ClientName())
		}
		if auth.Key() != "test-key-base32" {
			t.Fatalf("expected Key test-key-base32, got %s", auth.Key())
		}
	})
}

func TestHiddenServiceConfigWithClientAuth(t *testing.T) {
	t.Run("should accept single client auth entry", func(t *testing.T) {
		auth := NewHiddenServiceAuth("alice", "alice-key")
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceClientAuth(auth),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		auths := cfg.ClientAuth()
		if len(auths) != 1 {
			t.Fatalf("expected 1 client auth entry, got %d", len(auths))
		}
		if auths[0].ClientName() != "alice" {
			t.Fatalf("expected ClientName alice, got %s", auths[0].ClientName())
		}
	})

	t.Run("should accept multiple client auth entries", func(t *testing.T) {
		auth1 := NewHiddenServiceAuth("alice", "alice-key")
		auth2 := NewHiddenServiceAuth("bob", "bob-key")
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceClientAuth(auth1, auth2),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		auths := cfg.ClientAuth()
		if len(auths) != 2 {
			t.Fatalf("expected 2 client auth entries, got %d", len(auths))
		}
	})

	t.Run("should reject client auth with empty client name", func(t *testing.T) {
		auth := NewHiddenServiceAuth("", "some-key")
		_, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceClientAuth(auth),
		)
		if err == nil {
			t.Fatalf("expected error when ClientAuth has empty client name")
		}
	})

	t.Run("should reject client auth with empty key", func(t *testing.T) {
		auth := NewHiddenServiceAuth("alice", "")
		_, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceClientAuth(auth),
		)
		if err == nil {
			t.Fatalf("expected error when ClientAuth has empty key")
		}
	})
}

func TestBuildAddOnionCommand(t *testing.T) {
	t.Run("should build ADD_ONION command with NEW key for new service", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		cmd := buildAddOnionCommand(cfg)
		if cmd != "ADD_ONION NEW:ED25519-V3 Port=80,127.0.0.1:8080" {
			t.Fatalf("unexpected command: %s", cmd)
		}
	})

	t.Run("should build ADD_ONION command with existing private key for reuse", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServicePrivateKey("TESTKEY123"),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		cmd := buildAddOnionCommand(cfg)
		if cmd != "ADD_ONION ED25519-V3:TESTKEY123 Port=80,127.0.0.1:8080" {
			t.Fatalf("unexpected command: %s", cmd)
		}
	})

	t.Run("should build ADD_ONION command with multiple ports sorted by virtual port", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(443, 8443),
			WithHiddenServicePort(80, 8080),
			WithHiddenServicePort(22, 2222),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		cmd := buildAddOnionCommand(cfg)
		expected := "ADD_ONION NEW:ED25519-V3 Port=22,127.0.0.1:2222 Port=80,127.0.0.1:8080 Port=443,127.0.0.1:8443"
		if cmd != expected {
			t.Fatalf("expected command:\n%s\ngot:\n%s", expected, cmd)
		}
	})

	t.Run("should build ADD_ONION command with client auth entries", func(t *testing.T) {
		auth := NewHiddenServiceAuth("alice", "alice-key-base32")
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceClientAuth(auth),
		)
		if err != nil {
			t.Fatalf("NewHiddenServiceConfig returned error: %v", err)
		}
		cmd := buildAddOnionCommand(cfg)
		expected := "ADD_ONION NEW:ED25519-V3 Port=80,127.0.0.1:8080 ClientAuth=alice:alice-key-base32"
		if cmd != expected {
			t.Fatalf("expected command:\n%s\ngot:\n%s", expected, cmd)
		}
	})
}

func TestHiddenServiceConfigAccessors(t *testing.T) {
	t.Run("should return private key from config", func(t *testing.T) {
		privateKey := "ED25519-V3:ABC123"
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServicePrivateKey(privateKey),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		if cfg.PrivateKey() != privateKey {
			t.Errorf("expected private key %s, got %s", privateKey, cfg.PrivateKey())
		}
	})

	t.Run("should return empty string when no private key set", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		if cfg.PrivateKey() != "" {
			t.Errorf("expected empty private key, got %s", cfg.PrivateKey())
		}
	})

	t.Run("should return port mappings from config", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServicePort(443, 8443),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		ports := cfg.Ports()
		if len(ports) != 2 {
			t.Errorf("expected 2 ports, got %d", len(ports))
		}
	})

	t.Run("should return client auth from config", func(t *testing.T) {
		auth := NewHiddenServiceAuth("alice", "descriptor:x25519:ABC123")
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
			WithHiddenServiceClientAuth(auth),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		auths := cfg.ClientAuth()
		if len(auths) != 1 {
			t.Errorf("expected 1 client auth, got %d", len(auths))
		}
		if auths[0].ClientName() != "alice" {
			t.Errorf("expected client name 'alice', got %s", auths[0].ClientName())
		}
	})

	t.Run("should return empty client auth when none set", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		auths := cfg.ClientAuth()
		if len(auths) != 0 {
			t.Errorf("expected 0 client auths, got %d", len(auths))
		}
	})
}

func TestSaveAndLoadPrivateKey(t *testing.T) {
	t.Run("should save and load private key", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "test.key")

		hs := &hiddenService{
			privateKey: "ED25519-V3:testprivatekeydata",
		}

		err := hs.SavePrivateKey(keyPath)
		if err != nil {
			t.Fatalf("SavePrivateKey failed: %v", err)
		}

		// Verify file exists with correct permissions
		info, err := os.Stat(keyPath)
		if err != nil {
			t.Fatalf("key file not created: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
		}

		// Load the key back
		loadedKey, err := LoadPrivateKey(keyPath)
		if err != nil {
			t.Fatalf("LoadPrivateKey failed: %v", err)
		}
		if loadedKey != "ED25519-V3:testprivatekeydata" {
			t.Errorf("expected key to match, got %s", loadedKey)
		}
	})

	t.Run("should return error for empty private key", func(t *testing.T) {
		hs := &hiddenService{privateKey: ""}
		err := hs.SavePrivateKey("/tmp/test.key")
		if err == nil {
			t.Error("expected error for empty private key")
		}
	})

	t.Run("should return error for non-existent file", func(t *testing.T) {
		_, err := LoadPrivateKey("/nonexistent/path/key")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}

func TestWithHiddenServicePrivateKeyFile(t *testing.T) {
	t.Run("should load key from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "test.key")

		// Write a test key
		err := os.WriteFile(keyPath, []byte("ED25519-V3:testkey"), 0600)
		if err != nil {
			t.Fatalf("failed to write test key: %v", err)
		}

		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePrivateKeyFile(keyPath),
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		if cfg.PrivateKey() != "ED25519-V3:testkey" {
			t.Errorf("expected key to be loaded, got %s", cfg.PrivateKey())
		}
	})

	t.Run("should ignore non-existent file", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServicePrivateKeyFile("/nonexistent/key"),
			WithHiddenServicePort(80, 8080),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		if cfg.PrivateKey() != "" {
			t.Errorf("expected empty key for non-existent file, got %s", cfg.PrivateKey())
		}
	})
}

func TestPortHelpers(t *testing.T) {
	t.Run("WithHiddenServiceSamePort should map port to itself", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServiceSamePort(8080),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		ports := cfg.Ports()
		if ports[8080] != 8080 {
			t.Errorf("expected port 8080 mapped to 8080, got %d", ports[8080])
		}
	})

	t.Run("WithHiddenServiceHTTP should map port 80", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServiceHTTP(3000),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		ports := cfg.Ports()
		if ports[80] != 3000 {
			t.Errorf("expected port 80 mapped to 3000, got %d", ports[80])
		}
	})

	t.Run("WithHiddenServiceHTTPS should map port 443", func(t *testing.T) {
		cfg, err := NewHiddenServiceConfig(
			WithHiddenServiceHTTPS(3443),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		ports := cfg.Ports()
		if ports[443] != 3443 {
			t.Errorf("expected port 443 mapped to 3443, got %d", ports[443])
		}
	})
}

func TestWithHiddenServicePortNilMap(t *testing.T) {
	t.Run("should initialize nil targetPort map", func(t *testing.T) {
		cfg := &HiddenServiceConfig{targetPort: nil}
		opt := WithHiddenServicePort(80, 8080)
		opt(cfg)
		if cfg.targetPort[80] != 8080 {
			t.Errorf("expected port 80 mapped to 8080, got %d", cfg.targetPort[80])
		}
	})
}

func TestWithHiddenServicePortsNilMap(t *testing.T) {
	t.Run("should initialize nil targetPort map", func(t *testing.T) {
		cfg := &HiddenServiceConfig{targetPort: nil}
		opt := WithHiddenServicePorts(map[int]int{80: 8080, 443: 8443})
		opt(cfg)
		if cfg.targetPort[80] != 8080 {
			t.Errorf("expected port 80 mapped to 8080, got %d", cfg.targetPort[80])
		}
		if cfg.targetPort[443] != 8443 {
			t.Errorf("expected port 443 mapped to 8443, got %d", cfg.targetPort[443])
		}
	})
}

func TestHiddenServiceRemoveNilContext(t *testing.T) {
	t.Run("should handle nil context", func(_ *testing.T) {
		// Create a mock control client
		hs := &hiddenService{
			control: &ControlClient{authenticated: false},
			address: "test.onion",
		}
		// This will fail at authentication, but proves nil ctx is handled
		//nolint:staticcheck,errcheck,gosec // testing nil context handling
		hs.Remove(nil)
		// Error is expected because we don't have a real connection
		// but nil context should be handled without panic
	})
}

func TestValidateHiddenServiceConfigEmptyKeyType(t *testing.T) {
	t.Run("should reject empty key type", func(t *testing.T) {
		cfg := HiddenServiceConfig{
			keyType:    "",
			targetPort: map[int]int{80: 8080},
		}
		err := validateHiddenServiceConfig(cfg)
		if err == nil {
			t.Error("expected error for empty key type")
		}
	})
}

func TestGetHiddenServiceStatus(t *testing.T) {
	t.Run("should fail when not authenticated", func(t *testing.T) {
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 1024)
			_, _ = conn.Read(buf)                                   //nolint:errcheck
			_, _ = conn.Write([]byte("515 Bad authentication\r\n")) //nolint:errcheck
		}()

		client, err := NewControlClient(listener.Addr().String(), ControlAuth{}, 2*time.Second)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		_, err = client.GetHiddenServiceStatus(context.Background())
		if err == nil {
			t.Error("expected authentication error")
		}
	})

	t.Run("should return services on success", func(t *testing.T) {
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 1024)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					return
				}
				command := string(buf[:n])
				if strings.Contains(command, "AUTHENTICATE") {
					_, _ = conn.Write([]byte("250 OK\r\n")) //nolint:errcheck
					continue
				}
				if strings.Contains(command, "GETINFO onions/current") {
					_, _ = conn.Write([]byte("250-onions/current=abc123\r\n250 OK\r\n")) //nolint:errcheck
					return
				}
			}
		}()

		client, err := NewControlClient(listener.Addr().String(), ControlAuth{}, 2*time.Second)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		services, err := client.GetHiddenServiceStatus(context.Background())
		if err != nil {
			t.Fatalf("GetHiddenServiceStatus failed: %v", err)
		}
		if len(services) != 1 {
			t.Errorf("expected 1 service, got %d", len(services))
		}
	})

	t.Run("should return empty list on error", func(t *testing.T) {
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 1024)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					return
				}
				command := string(buf[:n])
				if strings.Contains(command, "AUTHENTICATE") {
					_, _ = conn.Write([]byte("250 OK\r\n")) //nolint:errcheck
					continue
				}
				if strings.Contains(command, "GETINFO onions/current") {
					_, _ = conn.Write([]byte("552 Unrecognized key\r\n")) //nolint:errcheck
					return
				}
			}
		}()

		client, err := NewControlClient(listener.Addr().String(), ControlAuth{}, 2*time.Second)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		services, err := client.GetHiddenServiceStatus(context.Background())
		if err != nil {
			t.Fatalf("GetHiddenServiceStatus should not return error: %v", err)
		}
		if len(services) != 0 {
			t.Errorf("expected 0 services, got %d", len(services))
		}
	})

	t.Run("should handle empty onions/current", func(t *testing.T) {
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			buf := make([]byte, 1024)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					return
				}
				command := string(buf[:n])
				if strings.Contains(command, "AUTHENTICATE") {
					_, _ = conn.Write([]byte("250 OK\r\n")) //nolint:errcheck
					continue
				}
				if strings.Contains(command, "GETINFO onions/current") {
					_, _ = conn.Write([]byte("250-onions/current=\r\n250 OK\r\n")) //nolint:errcheck
					return
				}
			}
		}()

		client, err := NewControlClient(listener.Addr().String(), ControlAuth{}, 2*time.Second)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		services, err := client.GetHiddenServiceStatus(context.Background())
		if err != nil {
			t.Fatalf("GetHiddenServiceStatus failed: %v", err)
		}
		if len(services) != 0 {
			t.Errorf("expected 0 services, got %d", len(services))
		}
	})
}
