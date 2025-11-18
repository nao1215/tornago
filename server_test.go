package tornago

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	t.Run("should create server with default configuration", func(t *testing.T) {
		cfg, err := NewServerConfig()
		if err != nil {
			t.Fatalf("NewServerConfig failed: %v", err)
		}

		server, err := NewServer(cfg)
		if err != nil {
			t.Fatalf("NewServer failed: %v", err)
		}

		if server == nil {
			t.Fatal("NewServer returned nil")
		}

		if server.SocksAddr() == "" {
			t.Error("SocksAddr should not be empty")
		}

		if server.ControlAddr() == "" {
			t.Error("ControlAddr should not be empty")
		}
	})

	t.Run("should create server with custom addresses", func(t *testing.T) {
		cfg, err := NewServerConfig(
			WithServerSocksAddr("127.0.0.1:19050"),
			WithServerControlAddr("127.0.0.1:19051"),
		)
		if err != nil {
			t.Fatalf("NewServerConfig failed: %v", err)
		}

		server, err := NewServer(cfg)
		if err != nil {
			t.Fatalf("NewServer failed: %v", err)
		}

		if server.SocksAddr() != "127.0.0.1:19050" {
			t.Errorf("expected SocksAddr 127.0.0.1:19050, got %s", server.SocksAddr())
		}

		if server.ControlAddr() != "127.0.0.1:19051" {
			t.Errorf("expected ControlAddr 127.0.0.1:19051, got %s", server.ControlAddr())
		}
	})
}

func TestServerConfig(t *testing.T) {
	t.Run("should return configured socks address", func(t *testing.T) {
		cfg, err := NewServerConfig(
			WithServerSocksAddr("192.168.1.1:9050"),
		)
		if err != nil {
			t.Fatalf("NewServerConfig failed: %v", err)
		}

		if cfg.SocksAddr() != "192.168.1.1:9050" {
			t.Errorf("expected SocksAddr 192.168.1.1:9050, got %s", cfg.SocksAddr())
		}
	})

	t.Run("should return configured control address", func(t *testing.T) {
		cfg, err := NewServerConfig(
			WithServerControlAddr("192.168.1.1:9051"),
		)
		if err != nil {
			t.Fatalf("NewServerConfig failed: %v", err)
		}

		if cfg.ControlAddr() != "192.168.1.1:9051" {
			t.Errorf("expected ControlAddr 192.168.1.1:9051, got %s", cfg.ControlAddr())
		}
	})
}
