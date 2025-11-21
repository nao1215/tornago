package tornago

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestOnionAddr(t *testing.T) {
	t.Parallel()

	addr := &OnionAddr{
		address: "abc123.onion:80",
		port:    80,
	}

	t.Run("Network returns onion", func(t *testing.T) {
		t.Parallel()
		if got := addr.Network(); got != "onion" {
			t.Errorf("Network() = %q, want %q", got, "onion")
		}
	})

	t.Run("String returns full address", func(t *testing.T) {
		t.Parallel()
		if got := addr.String(); got != "abc123.onion:80" {
			t.Errorf("String() = %q, want %q", got, "abc123.onion:80")
		}
	})

	t.Run("Port returns port number", func(t *testing.T) {
		t.Parallel()
		if got := addr.Port(); got != 80 {
			t.Errorf("Port() = %d, want %d", got, 80)
		}
	})
}

func TestOnionAddrImplementsNetAddr(t *testing.T) {
	t.Parallel()

	var _ net.Addr = (*OnionAddr)(nil)
}

func TestTorListenerImplementsNetListener(t *testing.T) {
	t.Parallel()

	var _ net.Listener = (*TorListener)(nil)
}

func TestTorListener_AcceptOnClosed(t *testing.T) {
	t.Parallel()

	// Create a minimal listener for testing.
	listener := &TorListener{
		closed: true,
		onionAddr: &OnionAddr{
			address: "test.onion:80",
			port:    80,
		},
	}

	_, err := listener.Accept()
	if err == nil {
		t.Error("Accept() on closed listener should return error")
	}

	var torErr *TornagoError
	if !errors.As(err, &torErr) {
		t.Error("error should be TornagoError")
	}
	if torErr.Kind != ErrListenerClosed {
		t.Errorf("error kind = %v, want %v", torErr.Kind, ErrListenerClosed)
	}
}

func TestTorListener_CloseIdempotent(t *testing.T) {
	t.Parallel()

	// Create a minimal listener without underlying resources.
	listener := &TorListener{
		closed: false,
		onionAddr: &OnionAddr{
			address: "test.onion:80",
			port:    80,
		},
	}

	// First close should succeed.
	err := listener.Close()
	if err != nil {
		t.Errorf("first Close() returned error: %v", err)
	}

	// Second close should be no-op.
	err = listener.Close()
	if err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}
}

func TestTorListener_Addr(t *testing.T) {
	t.Parallel()

	onionAddr := &OnionAddr{
		address: "xyz789.onion:443",
		port:    443,
	}
	listener := &TorListener{
		onionAddr: onionAddr,
	}

	addr := listener.Addr()
	if addr != onionAddr {
		t.Errorf("Addr() = %v, want %v", addr, onionAddr)
	}
}

func TestTorListener_VirtualPort(t *testing.T) {
	t.Parallel()

	listener := &TorListener{
		virtualPort: 8080,
	}

	if got := listener.VirtualPort(); got != 8080 {
		t.Errorf("VirtualPort() = %d, want %d", got, 8080)
	}
}

func TestTorListener_OnionAddressNil(t *testing.T) {
	t.Parallel()

	listener := &TorListener{
		hiddenService: nil,
	}

	if got := listener.OnionAddress(); got != "" {
		t.Errorf("OnionAddress() with nil service = %q, want empty", got)
	}
}

func TestClient_ListenWithoutControl(t *testing.T) {
	t.Parallel()

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
	defer client.Close()

	_, err = client.Listen(context.Background(), 80, 8080)
	if err == nil {
		t.Error("Listen() without ControlClient should return error")
	}

	var torErr *TornagoError
	if !errors.As(err, &torErr) {
		t.Error("error should be TornagoError")
	}
	if torErr.Kind != ErrInvalidConfig {
		t.Errorf("error kind = %v, want %v", torErr.Kind, ErrInvalidConfig)
	}
}

func TestClient_ListenWithConfigWithoutControl(t *testing.T) {
	t.Parallel()

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
	defer client.Close()

	hsCfg, err := NewHiddenServiceConfig(
		WithHiddenServicePort(80, 8080),
	)
	if err != nil {
		t.Fatalf("NewHiddenServiceConfig failed: %v", err)
	}

	_, err = client.ListenWithConfig(context.Background(), hsCfg, 8080)
	if err == nil {
		t.Error("ListenWithConfig() without ControlClient should return error")
	}

	var torErr *TornagoError
	if !errors.As(err, &torErr) {
		t.Error("error should be TornagoError")
	}
	if torErr.Kind != ErrInvalidConfig {
		t.Errorf("error kind = %v, want %v", torErr.Kind, ErrInvalidConfig)
	}
}

func TestTorListener_HiddenService(t *testing.T) {
	t.Parallel()

	// Test with nil hidden service.
	listener := &TorListener{
		hiddenService: nil,
	}
	if got := listener.HiddenService(); got != nil {
		t.Errorf("HiddenService() = %v, want nil", got)
	}
}

func TestTorListener_OnionAddressWithService(t *testing.T) {
	t.Parallel()

	// Create a mock hidden service for testing.
	mockHS := &mockHiddenService{
		address: "test123.onion",
	}
	listener := &TorListener{
		hiddenService: mockHS,
	}

	if got := listener.OnionAddress(); got != "test123.onion" {
		t.Errorf("OnionAddress() = %q, want %q", got, "test123.onion")
	}
}

func TestTorListener_AcceptSuccess(t *testing.T) {
	t.Parallel()

	// Create a real TCP listener for testing.
	lc := &net.ListenConfig{}
	tcpListener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create TCP listener: %v", err)
	}
	defer tcpListener.Close()

	listener := &TorListener{
		underlying: tcpListener,
		onionAddr: &OnionAddr{
			address: "test.onion:80",
			port:    80,
		},
	}

	// Create a connection to accept.
	go func() {
		d := &net.Dialer{}
		conn, err := d.DialContext(context.Background(), "tcp", tcpListener.Addr().String())
		if err != nil {
			return
		}
		_ = conn.Close()
	}()

	conn, err := listener.Accept()
	if err != nil {
		t.Errorf("Accept() returned error: %v", err)
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func TestTorListener_CloseWithUnderlying(t *testing.T) {
	t.Parallel()

	// Create a real TCP listener for testing.
	lc := &net.ListenConfig{}
	tcpListener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create TCP listener: %v", err)
	}

	listener := &TorListener{
		underlying: tcpListener,
		onionAddr: &OnionAddr{
			address: "test.onion:80",
			port:    80,
		},
	}

	err = listener.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Verify listener is closed.
	if !listener.closed {
		t.Error("listener should be marked as closed")
	}
}

// mockHiddenService implements HiddenService for testing.
type mockHiddenService struct {
	address    string
	privateKey string
	ports      map[int]int
	auth       []HiddenServiceAuth
	removeErr  error
}

func (m *mockHiddenService) OnionAddress() string {
	return m.address
}

func (m *mockHiddenService) PrivateKey() string {
	return m.privateKey
}

func (m *mockHiddenService) Ports() map[int]int {
	return m.ports
}

func (m *mockHiddenService) ClientAuth() []HiddenServiceAuth {
	return m.auth
}

func (m *mockHiddenService) Remove(_ context.Context) error {
	return m.removeErr
}

func (m *mockHiddenService) SavePrivateKey(_ string) error {
	return nil
}

func TestTorListener_CloseWithHiddenService(t *testing.T) {
	t.Parallel()

	// Create a real TCP listener for testing.
	lc := &net.ListenConfig{}
	tcpListener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create TCP listener: %v", err)
	}

	mockHS := &mockHiddenService{
		address: "test.onion",
	}

	listener := &TorListener{
		underlying:    tcpListener,
		hiddenService: mockHS,
		onionAddr: &OnionAddr{
			address: "test.onion:80",
			port:    80,
		},
	}

	err = listener.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestTorListener_CloseWithHiddenServiceError(t *testing.T) {
	t.Parallel()

	// Create a real TCP listener for testing.
	lc := &net.ListenConfig{}
	tcpListener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create TCP listener: %v", err)
	}

	mockHS := &mockHiddenService{
		address:   "test.onion",
		removeErr: errors.New("remove failed"),
	}

	listener := &TorListener{
		underlying:    tcpListener,
		hiddenService: mockHS,
		onionAddr: &OnionAddr{
			address: "test.onion:80",
			port:    80,
		},
	}

	err = listener.Close()
	if err == nil {
		t.Error("Close() should return error when hidden service removal fails")
	}

	var torErr *TornagoError
	if !errors.As(err, &torErr) {
		t.Error("error should be TornagoError")
	}
	if torErr.Kind != ErrListenerCloseFailed {
		t.Errorf("error kind = %v, want %v", torErr.Kind, ErrListenerCloseFailed)
	}
}

func TestClient_Dialer(t *testing.T) {
	t.Parallel()

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
	defer client.Close()

	dialer := client.Dialer()
	if dialer == nil {
		t.Error("Dialer() returned nil")
	}
}

func TestClient_Metrics(t *testing.T) {
	t.Parallel()

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
	defer client.Close()

	// Without metrics configured, should return nil.
	if client.Metrics() != nil {
		t.Error("Metrics() should return nil when not configured")
	}
}
