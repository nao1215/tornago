package tornago

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("should create client with valid configuration", func(t *testing.T) {
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
		)
		if err != nil {
			t.Fatalf("NewClientConfig failed: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}

		if client == nil {
			t.Fatal("NewClient returned nil client")
		}

		// Test HTTP() method
		httpClient := client.HTTP()
		if httpClient == nil {
			t.Fatal("HTTP() returned nil")
		}

		// Test Control() method
		controlClient := client.Control()
		if controlClient != nil {
			// Control client should be nil when not configured
			t.Error("Control() should return nil when not configured")
		}

		// Clean up
		if err := client.Close(); err != nil {
			t.Errorf("Close() failed: %v", err)
		}
	})

	t.Run("should create client configuration with control port", func(t *testing.T) {
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithClientControlAddr("127.0.0.1:9051"),
			WithClientControlPassword("test-password"),
		)
		if err != nil {
			t.Fatalf("NewClientConfig failed: %v", err)
		}

		// Verify config has control port settings
		if cfg.ControlAddr() == "" {
			t.Error("ControlAddr should be set")
		}
		if cfg.ControlAuth().Password() != "test-password" {
			t.Error("ControlAuth password should be set")
		}
	})
}

func TestParsePort(t *testing.T) {
	t.Run("should parse valid port number", func(t *testing.T) {
		port, err := parsePort("80")
		if err != nil {
			t.Fatalf("parsePort failed: %v", err)
		}
		if port != 80 {
			t.Errorf("expected port 80, got %d", port)
		}
	})

	t.Run("should parse https default port", func(t *testing.T) {
		port, err := parsePort("443")
		if err != nil {
			t.Fatalf("parsePort failed: %v", err)
		}
		if port != 443 {
			t.Errorf("expected port 443, got %d", port)
		}
	})

	t.Run("should reject port number exceeding 65535", func(t *testing.T) {
		_, err := parsePort("99999")
		if err == nil {
			t.Error("parsePort should fail for port > 65535")
		}
	})

	t.Run("should reject negative port number", func(t *testing.T) {
		_, err := parsePort("-1")
		if err == nil {
			t.Error("parsePort should fail for negative port")
		}
	})

	t.Run("should reject non-numeric port", func(t *testing.T) {
		_, err := parsePort("abc")
		if err == nil {
			t.Error("parsePort should fail for non-numeric input")
		}
	})
}

func TestBuildConnectRequest(t *testing.T) {
	t.Run("should build CONNECT request for hostname and port", func(t *testing.T) {
		req, err := buildConnectRequest("example.com", 80)
		if err != nil {
			t.Fatalf("buildConnectRequest failed: %v", err)
		}
		if req == nil {
			t.Fatal("buildConnectRequest returned nil")
		}

		if len(req) < 10 {
			t.Error("CONNECT request too short")
		}

		// Check SOCKS5 version
		if req[0] != 0x05 {
			t.Errorf("expected SOCKS5 version 0x05, got 0x%02x", req[0])
		}

		// Check command (CONNECT)
		if req[1] != 0x01 {
			t.Errorf("expected CONNECT command 0x01, got 0x%02x", req[1])
		}

		// Check address type (domain name)
		if req[3] != 0x03 {
			t.Errorf("expected domain name type 0x03, got 0x%02x", req[3])
		}
	})

	t.Run("should build CONNECT request for IPv4 address", func(t *testing.T) {
		req, err := buildConnectRequest("192.168.1.1", 8080)
		if err != nil {
			t.Fatalf("buildConnectRequest failed: %v", err)
		}
		if req == nil {
			t.Fatal("buildConnectRequest returned nil")
		}

		// Check address type (IPv4)
		if req[3] != 0x01 {
			t.Errorf("expected IPv4 type 0x01, got 0x%02x", req[3])
		}
	})

	t.Run("should build CONNECT request for IPv6 address", func(t *testing.T) {
		req, err := buildConnectRequest("::1", 8080)
		if err != nil {
			t.Fatalf("buildConnectRequest failed: %v", err)
		}
		if req == nil {
			t.Fatal("buildConnectRequest returned nil")
		}

		// Check address type (IPv6)
		if req[3] != 0x04 {
			t.Errorf("expected IPv6 type 0x04, got 0x%02x", req[3])
		}
	})
}

func TestCloneRequestWithContext(t *testing.T) {
	t.Run("should clone HTTP request with new context", func(t *testing.T) {
		originalReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		originalReq.Header.Set("User-Agent", "test-agent")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		clonedReq, err := cloneRequestWithContext(ctx, originalReq)
		if err != nil {
			t.Fatalf("cloneRequestWithContext failed: %v", err)
		}

		if clonedReq == nil {
			t.Fatal("cloneRequestWithContext returned nil")
		}

		if clonedReq.URL.String() != originalReq.URL.String() {
			t.Error("cloned request URL doesn't match original")
		}

		if clonedReq.Header.Get("User-Agent") != "test-agent" {
			t.Error("cloned request headers don't match original")
		}

		if clonedReq.Context() == originalReq.Context() {
			t.Error("cloned request context should be different from original")
		}
	})
}

func TestWriteAll(t *testing.T) {
	t.Run("should write all bytes to connection", func(t *testing.T) {
		// Create a pipe to simulate a network connection
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		data := []byte("test data")
		done := make(chan error, 1)

		// Write in goroutine
		go func() {
			done <- writeAll(client, data)
		}()

		// Read from server side
		buf := make([]byte, len(data))
		n, err := server.Read(buf)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}

		if n != len(data) {
			t.Errorf("expected to read %d bytes, got %d", len(data), n)
		}

		if string(buf) != string(data) {
			t.Errorf("expected %q, got %q", data, buf)
		}

		// Check write error
		if err := <-done; err != nil {
			t.Errorf("writeAll failed: %v", err)
		}
	})
}

func TestConsumeConnectReply(t *testing.T) {
	t.Run("should successfully consume valid SOCKS5 reply", func(t *testing.T) {
		// Create a pipe
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		// Send valid SOCKS5 reply in goroutine
		go func() {
			// Version, status (success), reserved, address type (IPv4)
			reply := []byte{0x05, 0x00, 0x00, 0x01}
			// IPv4 address (4 bytes) + port (2 bytes)
			reply = append(reply, []byte{0, 0, 0, 0, 0, 0}...)
			_, _ = server.Write(reply) //nolint:errcheck
		}()

		err := consumeConnectReply(client)
		if err != nil {
			t.Errorf("consumeConnectReply failed: %v", err)
		}
	})

	t.Run("should handle domain name address type", func(t *testing.T) {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		go func() {
			// Version, status, reserved, address type (domain)
			reply := []byte{0x05, 0x00, 0x00, 0x03}
			// Domain length + domain name + port
			reply = append(reply, byte(11)) // length of "example.com"
			reply = append(reply, []byte("example.com")...)
			reply = append(reply, 0, 80) // port 80
			_, _ = server.Write(reply)   //nolint:errcheck
		}()

		err := consumeConnectReply(client)
		if err != nil {
			t.Errorf("consumeConnectReply failed for domain: %v", err)
		}
	})

	t.Run("should reject non-zero status code", func(t *testing.T) {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		go func() {
			// Version, status (general failure), reserved, address type
			reply := []byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
			_, _ = server.Write(reply) //nolint:errcheck
		}()

		err := consumeConnectReply(client)
		if err == nil {
			t.Error("consumeConnectReply should fail for non-zero status")
		}
	})
}

func TestBuildConnectRequestIPv6(t *testing.T) {
	t.Run("should build CONNECT request for IPv6 address", func(t *testing.T) {
		req, err := buildConnectRequest("::1", 80)
		if err != nil {
			t.Fatalf("buildConnectRequest failed: %v", err)
		}
		if len(req) == 0 {
			t.Error("expected non-empty request")
		}
		// Check SOCKS5 version
		if req[0] != 0x05 {
			t.Errorf("expected SOCKS5 version 0x05, got 0x%02x", req[0])
		}
		// Check CONNECT command
		if req[1] != 0x01 {
			t.Errorf("expected CONNECT command 0x01, got 0x%02x", req[1])
		}
		// Check address type (IPv6 = 0x04)
		if req[3] != 0x04 {
			t.Errorf("expected IPv6 address type 0x04, got 0x%02x", req[3])
		}
	})
}

func TestBuildConnectRequestIPv4(t *testing.T) {
	t.Run("should build CONNECT request for IPv4 address", func(t *testing.T) {
		req, err := buildConnectRequest("192.168.1.1", 8080)
		if err != nil {
			t.Fatalf("buildConnectRequest failed: %v", err)
		}
		if len(req) == 0 {
			t.Error("expected non-empty request")
		}
		// Check SOCKS5 version
		if req[0] != 0x05 {
			t.Errorf("expected SOCKS5 version 0x05, got 0x%02x", req[0])
		}
		// Check CONNECT command
		if req[1] != 0x01 {
			t.Errorf("expected CONNECT command 0x01, got 0x%02x", req[1])
		}
		// Check address type (IPv4 = 0x01)
		if req[3] != 0x01 {
			t.Errorf("expected IPv4 address type 0x01, got 0x%02x", req[3])
		}
	})
}

func TestClientDial(t *testing.T) {
	t.Run("should dial through SOCKS5 proxy", func(t *testing.T) {
		// Create a mock SOCKS5 server
		mockSOCKS := createMockSOCKS5Server(t)
		defer mockSOCKS.Close()

		cfg, err := NewClientConfig(
			WithClientSocksAddr(mockSOCKS.Addr().String()),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		// Test Dial
		conn, err := client.Dial("tcp", "example.com:80")
		if err != nil {
			t.Fatalf("Dial failed: %v", err)
		}
		if conn != nil {
			_ = conn.Close()
		}
	})

	t.Run("should fail dial with invalid address", func(t *testing.T) {
		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9999"), // Non-existent SOCKS proxy
			WithClientDialTimeout(100*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		// Should fail to dial
		_, err = client.Dial("tcp", "example.com:80")
		if err == nil {
			t.Error("expected Dial to fail with non-existent SOCKS proxy")
		}
	})
}

func TestClientDo(t *testing.T) {
	t.Run("should make HTTP request through SOCKS5", func(t *testing.T) {
		// Create a test HTTP server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response")) //nolint:errcheck
		}))
		defer testServer.Close()

		// Create a mock SOCKS5 server that forwards to the test server
		mockSOCKS := createMockSOCKS5ServerWithForwarding(t, testServer.Listener.Addr().String())
		defer mockSOCKS.Close()

		cfg, err := NewClientConfig(
			WithClientSocksAddr(mockSOCKS.Addr().String()),
			WithClientRequestTimeout(5*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		// Make HTTP request
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, testServer.URL, nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Do failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if string(body) != "test response" {
			t.Errorf("expected 'test response', got %s", string(body))
		}
	})

	t.Run("should handle request context cancellation", func(t *testing.T) {
		mockSOCKS := createMockSOCKS5Server(t)
		defer mockSOCKS.Close()

		cfg, err := NewClientConfig(
			WithClientSocksAddr(mockSOCKS.Addr().String()),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		// Create a request with a cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			t.Error("expected Do to fail with cancelled context")
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	})
}

func TestCloneRequestWithContextAdditional(t *testing.T) {
	t.Run("should preserve request body when cloning", func(t *testing.T) {
		body := strings.NewReader("test body content")
		originalReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com", body)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		originalReq.Header.Set("Content-Type", "text/plain")

		ctx := context.Background()
		clonedReq, err := cloneRequestWithContext(ctx, originalReq)
		if err != nil {
			t.Fatalf("cloneRequestWithContext failed: %v", err)
		}

		// Verify body is preserved
		if clonedReq.Body == nil {
			t.Error("cloned request should have body")
		}

		// Verify headers are preserved
		if clonedReq.Header.Get("Content-Type") != "text/plain" {
			t.Error("cloned request should preserve headers")
		}

		// Verify method is preserved
		if clonedReq.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", clonedReq.Method)
		}
	})

	t.Run("should handle nil body", func(t *testing.T) {
		originalReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		ctx := context.Background()
		clonedReq, err := cloneRequestWithContext(ctx, originalReq)
		if err != nil {
			t.Fatalf("cloneRequestWithContext failed: %v", err)
		}

		if clonedReq.Body != nil {
			t.Error("cloned request should have nil body when original has nil body")
		}
	})

	t.Run("should use provided context", func(t *testing.T) {
		originalReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		clonedReq, err := cloneRequestWithContext(ctx, originalReq)
		if err != nil {
			t.Fatalf("cloneRequestWithContext failed: %v", err)
		}

		if clonedReq.Context() != ctx {
			t.Error("cloned request should use provided context")
		}
	})
}

func TestDialWithSOCKS5Handshake(t *testing.T) {
	t.Run("should fail with wrong SOCKS5 version response", func(t *testing.T) {
		// Create a mock server that sends wrong version
		mockSOCKS := createMockSOCKS5ServerWithWrongVersion(t)
		defer mockSOCKS.Close()

		cfg, err := NewClientConfig(
			WithClientSocksAddr(mockSOCKS.Addr().String()),
			WithClientDialTimeout(1*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		_, err = client.Dial("tcp", "example.com:80")
		if err == nil {
			t.Error("expected Dial to fail with wrong SOCKS version")
		}
	})

	t.Run("should fail when SOCKS server requires auth", func(t *testing.T) {
		// Create a mock server that requires authentication
		mockSOCKS := createMockSOCKS5ServerRequiringAuth(t)
		defer mockSOCKS.Close()

		cfg, err := NewClientConfig(
			WithClientSocksAddr(mockSOCKS.Addr().String()),
			WithClientDialTimeout(1*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		_, err = client.Dial("tcp", "example.com:80")
		if err == nil {
			t.Error("expected Dial to fail when auth is required")
		}
	})

	t.Run("should handle SOCKS server connection errors", func(t *testing.T) {
		// Create a mock server that immediately closes connections
		mockSOCKS := createMockSOCKS5ServerClosingImmediately(t)
		defer mockSOCKS.Close()

		cfg, err := NewClientConfig(
			WithClientSocksAddr(mockSOCKS.Addr().String()),
			WithClientDialTimeout(1*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		defer client.Close()

		_, err = client.Dial("tcp", "example.com:80")
		if err == nil {
			t.Error("expected Dial to fail with immediate connection close")
		}
	})
}

// Mock SOCKS5 server for testing
type mockSOCKS5Server struct {
	listener net.Listener
	done     chan struct{}
}

func (m *mockSOCKS5Server) Addr() net.Addr {
	return m.listener.Addr()
}

func (m *mockSOCKS5Server) Close() {
	close(m.done)
	_ = m.listener.Close()
}

func createMockSOCKS5Server(t *testing.T) *mockSOCKS5Server {
	t.Helper()
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	mock := &mockSOCKS5Server{
		listener: listener,
		done:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-mock.done:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go handleMockSOCKS5Connection(conn)
			}
		}
	}()

	return mock
}

func createMockSOCKS5ServerWithForwarding(t *testing.T, targetAddr string) *mockSOCKS5Server {
	t.Helper()
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	mock := &mockSOCKS5Server{
		listener: listener,
		done:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-mock.done:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go handleMockSOCKS5ConnectionWithForwarding(conn, targetAddr)
			}
		}
	}()

	return mock
}

func handleMockSOCKS5Connection(conn net.Conn) {
	defer conn.Close()

	// Read and respond to SOCKS5 greeting
	buf := make([]byte, 258)
	n, err := conn.Read(buf)
	if err != nil || n < 3 {
		return
	}

	// Send method selection (no auth)
	_, _ = conn.Write([]byte{0x05, 0x00}) //nolint:errcheck

	// Read CONNECT request
	n, err = conn.Read(buf)
	if err != nil || n < 10 {
		return
	}

	// Send success reply
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	_, _ = conn.Write(reply) //nolint:errcheck

	// Keep connection open for a bit
	time.Sleep(100 * time.Millisecond)
}

func handleMockSOCKS5ConnectionWithForwarding(clientConn net.Conn, targetAddr string) {
	defer clientConn.Close()

	// Read and respond to SOCKS5 greeting
	buf := make([]byte, 258)
	n, err := clientConn.Read(buf)
	if err != nil || n < 3 {
		return
	}

	// Send method selection (no auth)
	_, _ = clientConn.Write([]byte{0x05, 0x00}) //nolint:errcheck

	// Read CONNECT request
	n, err = clientConn.Read(buf)
	if err != nil || n < 10 {
		return
	}

	// Connect to target
	dialer := net.Dialer{}
	targetConn, err := dialer.DialContext(context.Background(), "tcp", targetAddr)
	if err != nil {
		// Send failure reply
		reply := []byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
		_, _ = clientConn.Write(reply) //nolint:errcheck
		return
	}
	defer targetConn.Close()

	// Send success reply
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	_, _ = clientConn.Write(reply) //nolint:errcheck

	// Forward data between client and target
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(targetConn, clientConn) //nolint:errcheck
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(clientConn, targetConn) //nolint:errcheck
		done <- struct{}{}
	}()

	// Wait for one direction to finish
	<-done
}

func createMockSOCKS5ServerWithWrongVersion(t *testing.T) *mockSOCKS5Server {
	t.Helper()
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	mock := &mockSOCKS5Server{
		listener: listener,
		done:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-mock.done:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 258)
					_, _ = c.Read(buf) //nolint:errcheck
					// Send wrong version (SOCKS4)
					_, _ = c.Write([]byte{0x04, 0x00}) //nolint:errcheck
				}(conn)
			}
		}
	}()

	return mock
}

func createMockSOCKS5ServerRequiringAuth(t *testing.T) *mockSOCKS5Server {
	t.Helper()
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	mock := &mockSOCKS5Server{
		listener: listener,
		done:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-mock.done:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go func(c net.Conn) {
					defer c.Close()
					buf := make([]byte, 258)
					_, _ = c.Read(buf) //nolint:errcheck
					// Send auth required (method 0x02 = username/password)
					_, _ = c.Write([]byte{0x05, 0x02}) //nolint:errcheck
				}(conn)
			}
		}
	}()

	return mock
}

func createMockSOCKS5ServerClosingImmediately(t *testing.T) *mockSOCKS5Server {
	t.Helper()
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	mock := &mockSOCKS5Server{
		listener: listener,
		done:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-mock.done:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				// Close connection immediately
				_ = conn.Close()
			}
		}
	}()

	return mock
}

func TestNewClientWithControl(t *testing.T) {
	t.Run("should create client with control client when control address is set", func(t *testing.T) {
		// Create mock control server
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
			_ = conn.Close()
		}()

		cfg, err := NewClientConfig(
			WithClientSocksAddr("127.0.0.1:9050"),
			WithClientControlAddr(listener.Addr().String()),
		)
		if err != nil {
			t.Fatalf("failed to create config: %v", err)
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient failed: %v", err)
		}
		defer client.Close()

		if client.Control() == nil {
			t.Error("expected non-nil control client when control address is set")
		}
	})
}

func TestConsumeConnectReplyIPv6(t *testing.T) {
	t.Run("should handle IPv6 address in CONNECT reply", func(t *testing.T) {
		// Create a pipe to simulate connection
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		go func() {
			// Send SOCKS5 CONNECT reply with IPv6 address (0x04)
			reply := []byte{
				0x05,       // version
				0x00,       // success
				0x00,       // reserved
				0x04,       // IPv6 address type
				0, 0, 0, 0, // 16 bytes for IPv6
				0, 0, 0, 0,
				0, 0, 0, 0,
				0, 0, 0, 0,
				0, 0, // port
			}
			_, _ = server.Write(reply) //nolint:errcheck
		}()

		err := consumeConnectReply(client)
		if err != nil {
			t.Errorf("consumeConnectReply failed with IPv6: %v", err)
		}
	})
}
