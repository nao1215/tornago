package tornago

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	// opClient labels errors originating from Client operations.
	opClient = "Client"
)

// Client bundles HTTP/TCP over Tor along with optional ControlPort access.
// It rewrites Dial/HTTP operations to go through Tor's SOCKS5 proxy and
// automatically retries failures based on ClientConfig.
//
// Client is the main entry point for making HTTP requests or TCP connections through Tor.
// It handles:
//   - Automatic SOCKS5 proxying through Tor's SocksPort
//   - Exponential backoff retry logic for failed requests
//   - Optional ControlPort access for circuit rotation and hidden services
//   - Thread-safe operation for concurrent requests
//
// Example usage:
//
//	cfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	)
//	client, _ := tornago.NewClient(cfg)
//	defer client.Close()
//
//	// Make HTTP requests through Tor
//	resp, err := client.HTTP().Get("https://check.torproject.org")
//
//	// Make raw TCP connections through Tor
//	conn, err := client.Dial("tcp", "example.onion:80")
type Client struct {
	// httpClient issues HTTP requests routed through Tor.
	httpClient *http.Client
	// control holds the optional ControlPort client.
	control *ControlClient
	// cfg stores the normalized client configuration.
	cfg ClientConfig
	// socksDialer performs SOCKS5 CONNECT handshakes to Tor.
	socksDialer *socks5Dialer
	// retryPolicy controls retry behavior for dial/HTTP operations.
	retryPolicy retryPolicy
	// metrics collects request statistics (optional).
	metrics *MetricsCollector
	// rateLimiter controls request rate (optional).
	rateLimiter *RateLimiter
}

// NewClient builds a Client that routes traffic through the configured Tor server.
// The client is ready to use immediately after creation - all connections will
// automatically be routed through Tor's SOCKS5 proxy.
//
// If cfg includes a ControlAddr, the client will also connect to Tor's ControlPort
// for management operations (e.g., circuit rotation, hidden service creation).
//
// Always call Close() when done to clean up resources.
func NewClient(cfg ClientConfig) (*Client, error) {
	cfg, err := normalizeClientConfig(cfg)
	if err != nil {
		return nil, err
	}

	retry := retryPolicy{
		attempts:    cfg.RetryAttempts(),
		delay:       cfg.RetryDelay(),
		maxDelay:    cfg.RetryMaxDelay(),
		shouldRetry: cfg.RetryOnError(),
	}

	dialer := &socks5Dialer{
		addr:    cfg.SocksAddr(),
		timeout: cfg.DialTimeout(),
	}

	client := &Client{
		cfg:         cfg,
		socksDialer: dialer,
		retryPolicy: retry,
		metrics:     cfg.Metrics(),
		rateLimiter: cfg.RateLimiter(),
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return client.dialContext(ctx, network, address)
		},
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: cfg.DialTimeout(),
	}

	client.httpClient = &http.Client{
		Transport: transport,
		Timeout:   cfg.RequestTimeout(),
	}

	if cfg.ControlAddr() != "" {
		controlClient, err := NewControlClient(cfg.ControlAddr(), cfg.ControlAuth(), cfg.DialTimeout())
		if err != nil {
			return nil, err
		}
		client.control = controlClient
	}

	return client, nil
}

// HTTP returns the configured *http.Client that routes through Tor.
func (c *Client) HTTP() *http.Client {
	return c.httpClient
}

// Control returns the ControlClient, which may be nil if ControlAddr was empty.
func (c *Client) Control() *ControlClient {
	return c.control
}

// Metrics returns the metrics collector, which may be nil if not configured.
func (c *Client) Metrics() *MetricsCollector {
	return c.metrics
}

// Dial establishes a TCP connection via Tor's SOCKS5 proxy.
// This is equivalent to DialContext with context.Background().
func (c *Client) Dial(network, addr string) (net.Conn, error) {
	return c.DialContext(context.Background(), network, addr)
}

// DialContext establishes a TCP connection via Tor's SOCKS5 proxy with context support.
// The context can be used for cancellation and deadlines.
func (c *Client) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, newError(ErrSocksDialFailed, opClient, "rate limit wait failed", err)
		}
	}
	start := time.Now()
	var conn net.Conn
	err := c.withRetry(ctx, c.cfg.DialTimeout(), func(attemptCtx context.Context) error {
		var dialErr error
		conn, dialErr = c.socksDialer.DialContext(attemptCtx, network, addr)
		return dialErr
	})
	if c.metrics != nil {
		c.metrics.recordRequest(time.Since(start), err)
	}
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Dialer returns a net.Dialer-compatible function that routes connections through Tor.
// This can be used with libraries that accept a custom dial function.
//
// Example:
//
//	dialer := client.Dialer()
//	conn, err := dialer(ctx, "tcp", "example.onion:80")
func (c *Client) Dialer() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return c.DialContext
}

// Do performs an HTTP request via Tor with retry support.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, newError(ErrInvalidConfig, opClient, "request is nil", nil)
	}

	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(req.Context()); err != nil {
			return nil, newError(ErrHTTPFailed, opClient, "rate limit wait failed", err)
		}
	}

	start := time.Now()
	var resp *http.Response
	err := c.withRetry(req.Context(), 0, func(_ context.Context) error {
		cloned, cloneErr := cloneRequestWithContext(req.Context(), req)
		if cloneErr != nil {
			return newError(ErrHTTPFailed, opClient, "failed to clone request", cloneErr)
		}
		attemptResp, doErr := c.httpClient.Do(cloned)
		if doErr == nil {
			resp = attemptResp
			return nil
		}
		if attemptResp != nil && attemptResp.Body != nil {
			if closeErr := attemptResp.Body.Close(); closeErr != nil {
				doErr = errors.Join(doErr, closeErr)
			}
		}
		return newError(ErrHTTPFailed, opClient, "http request failed", doErr)
	})
	if c.metrics != nil {
		c.metrics.recordRequest(time.Since(start), err)
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Close closes the ControlClient and underlying HTTP transport resources.
func (c *Client) Close() error {
	var closeErr error
	if c.control != nil {
		closeErr = c.control.Close()
	}
	if c.httpClient != nil {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	return closeErr
}

// dialContext performs a SOCKS5 dial with retry logic.
func (c *Client) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var conn net.Conn
	err := c.withRetry(ctx, c.cfg.DialTimeout(), func(attemptCtx context.Context) error {
		var dialErr error
		conn, dialErr = c.socksDialer.DialContext(attemptCtx, network, addr)
		if dialErr != nil {
			return dialErr
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// withRetry executes fn according to the configured retry policy.
func (c *Client) withRetry(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	return c.retryPolicy.run(ctx, timeout, fn)
}

// retryPolicy implements simple exponential backoff retries.
type retryPolicy struct {
	// attempts is the maximum number of retries.
	attempts uint
	// delay is the initial backoff duration.
	delay time.Duration
	// maxDelay caps the backoff duration.
	maxDelay time.Duration
	// shouldRetry decides whether a specific error is retryable.
	shouldRetry func(error) bool
}

// run executes fn with exponential backoff respecting context cancellation.
func (p retryPolicy) run(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	attempts := p.attempts
	if attempts == 0 {
		attempts = 1
	}
	delay := p.delay
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	maxDelay := p.maxDelay
	if maxDelay < delay {
		maxDelay = delay
	}

	for i := uint(0); i < attempts; i++ { //nolint:intrange // explicit form for clarity with uint counters
		attemptCtx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		err := fn(attemptCtx)
		if cancel != nil {
			cancel()
		}
		if err == nil {
			return nil
		}
		if p.shouldRetry != nil && !p.shouldRetry(err) {
			return err
		}
		if i == attempts-1 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	return nil
}

// cloneRequestWithContext clones req and reinitializes its body for retries.
func cloneRequestWithContext(ctx context.Context, req *http.Request) (*http.Request, error) {
	cloned := req.Clone(ctx)
	if req.Body == nil || req.Body == http.NoBody {
		return cloned, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("request body is not replayable")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	cloned.Body = body
	return cloned, nil
}

// socks5Dialer performs minimal SOCKS5 CONNECT handshakes.
type socks5Dialer struct {
	// addr is the SOCKS5 proxy endpoint (Tor's SocksPort).
	addr string
	// timeout bounds dial operations to the proxy.
	timeout time.Duration
}

// DialContext establishes a SOCKS5 CONNECT tunnel for the destination address.
func (d *socks5Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, newError(ErrSocksDialFailed, opClient, "unsupported network "+network, nil)
	}

	dialer := &net.Dialer{}
	if d.timeout > 0 {
		dialer.Timeout = d.timeout
	}

	conn, err := dialer.DialContext(ctx, "tcp", d.addr)
	if err != nil {
		return nil, newError(ErrSocksDialFailed, opClient, "failed to connect to SOCKS proxy", err)
	}

	if err := d.handshake(conn, address); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, err
	}
	return conn, nil
}

// handshake performs the SOCKS5 CONNECT handshake to dest over conn.
func (d *socks5Dialer) handshake(conn net.Conn, dest string) error {
	if err := writeAll(conn, []byte{0x05, 0x01, 0x00}); err != nil {
		return newError(ErrSocksDialFailed, opClient, "failed to send greeting", err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		return newError(ErrSocksDialFailed, opClient, "failed to read greeting", err)
	}
	if reply[1] != 0x00 {
		return newError(ErrSocksDialFailed, opClient, "SOCKS authentication not accepted", nil)
	}

	host, portStr, err := net.SplitHostPort(dest)
	if err != nil {
		return newError(ErrSocksDialFailed, opClient, "invalid destination address", err)
	}
	port, err := parsePort(portStr)
	if err != nil {
		return newError(ErrSocksDialFailed, opClient, "invalid destination port", err)
	}

	req, err := buildConnectRequest(host, port)
	if err != nil {
		return err
	}
	if err := writeAll(conn, req); err != nil {
		return newError(ErrSocksDialFailed, opClient, "failed to send connect request", err)
	}

	if err := consumeConnectReply(conn); err != nil {
		return err
	}
	return nil
}

// writeAll writes the full buffer to w.
func writeAll(w io.Writer, b []byte) error {
	_, err := w.Write(b)
	return err
}

// parsePort converts a port string to uint16.
func parsePort(portStr string) (uint16, error) {
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, err
	}
	if p < 0 || p > 65535 {
		return 0, fmt.Errorf("port out of range: %d", p)
	}
	return uint16(p), nil
}

// buildConnectRequest builds a SOCKS5 CONNECT request for host:port.
func buildConnectRequest(host string, port uint16) ([]byte, error) {
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else if ip6 := ip.To16(); ip6 != nil {
			req = append(req, 0x04)
			req = append(req, ip6...)
		} else {
			return nil, newError(ErrSocksDialFailed, opClient, "invalid IP address", nil)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return nil, newError(ErrSocksDialFailed, opClient, "invalid hostname length", nil)
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	req = append(req, portBytes...)
	return req, nil
}

// Listen creates a TorListener that exposes a local TCP listener as a Tor Hidden Service.
// The virtualPort is the port exposed on the .onion address, and localPort is the local
// port that accepts connections.
//
// This method requires a ControlClient to be configured (via WithClientControlAddr).
//
// Example:
//
//	client, _ := tornago.NewClient(cfg)
//	listener, _ := client.Listen(ctx, 80, 8080) // onion:80 -> local:8080
//	defer listener.Close()
//
//	fmt.Printf("Listening at: %s\n", listener.OnionAddress())
//	for {
//	    conn, _ := listener.Accept()
//	    go handleConnection(conn)
//	}
func (c *Client) Listen(ctx context.Context, virtualPort, localPort int) (*TorListener, error) {
	if c.control == nil {
		return nil, newError(ErrInvalidConfig, opClient, "ControlClient is required for Listen", nil)
	}

	// Create local TCP listener.
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	lc := &net.ListenConfig{}
	underlying, err := lc.Listen(ctx, "tcp", localAddr)
	if err != nil {
		return nil, newError(ErrIO, opClient, "failed to create local listener", err)
	}

	// Get the actual port if localPort was 0 (auto-assign).
	actualPort := localPort
	if localPort == 0 {
		tcpAddr, ok := underlying.Addr().(*net.TCPAddr)
		if !ok {
			_ = underlying.Close()
			return nil, newError(ErrIO, opClient, "unexpected listener address type", nil)
		}
		actualPort = tcpAddr.Port
	}

	// Create hidden service configuration.
	hsCfg, err := NewHiddenServiceConfig(
		WithHiddenServicePort(virtualPort, actualPort),
	)
	if err != nil {
		_ = underlying.Close()
		return nil, err
	}

	// Create the hidden service.
	hs, err := c.control.CreateHiddenService(ctx, hsCfg)
	if err != nil {
		_ = underlying.Close()
		return nil, err
	}

	onionAddr := &OnionAddr{
		address: fmt.Sprintf("%s:%d", hs.OnionAddress(), virtualPort),
		port:    virtualPort,
	}

	return &TorListener{
		underlying:    underlying,
		hiddenService: hs,
		onionAddr:     onionAddr,
		virtualPort:   virtualPort,
	}, nil
}

// ListenWithConfig creates a TorListener using a custom HiddenServiceConfig.
// This allows for advanced configurations like persistent keys or client authorization.
//
// The HiddenServiceConfig must have exactly one port mapping, and its target port
// must match the localPort parameter.
//
// Example:
//
//	hsCfg, _ := tornago.NewHiddenServiceConfig(
//	    tornago.WithHiddenServicePrivateKey(savedKey),
//	    tornago.WithHiddenServicePort(80, 8080),
//	)
//	listener, _ := client.ListenWithConfig(ctx, hsCfg, 8080)
func (c *Client) ListenWithConfig(ctx context.Context, hsCfg HiddenServiceConfig, localPort int) (*TorListener, error) {
	if c.control == nil {
		return nil, newError(ErrInvalidConfig, opClient, "ControlClient is required for ListenWithConfig", nil)
	}

	// Validate port mapping: must have exactly one mapping and target port must match localPort.
	ports := hsCfg.Ports()
	if len(ports) != 1 {
		return nil, newError(ErrInvalidConfig, opClient, "HiddenServiceConfig must have exactly one port mapping for ListenWithConfig", nil)
	}
	var virtualPort, targetPort int
	for vp, tp := range ports {
		virtualPort, targetPort = vp, tp
	}
	if targetPort != localPort {
		return nil, newError(ErrInvalidConfig, opClient, "localPort must match hidden service target port", nil)
	}

	// Create local TCP listener.
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	lc := &net.ListenConfig{}
	underlying, err := lc.Listen(ctx, "tcp", localAddr)
	if err != nil {
		return nil, newError(ErrIO, opClient, "failed to create local listener", err)
	}

	// Create the hidden service.
	hs, err := c.control.CreateHiddenService(ctx, hsCfg)
	if err != nil {
		_ = underlying.Close()
		return nil, err
	}

	onionAddr := &OnionAddr{
		address: fmt.Sprintf("%s:%d", hs.OnionAddress(), virtualPort),
		port:    virtualPort,
	}

	return &TorListener{
		underlying:    underlying,
		hiddenService: hs,
		onionAddr:     onionAddr,
		virtualPort:   virtualPort,
	}, nil
}

// consumeConnectReply reads and validates the SOCKS5 CONNECT reply.
func consumeConnectReply(conn net.Conn) error {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return newError(ErrSocksDialFailed, opClient, "failed to read connect reply", err)
	}
	if header[1] != 0x00 {
		return newError(ErrSocksDialFailed, opClient, fmt.Sprintf("SOCKS connect failed: %d", header[1]), nil)
	}
	var addrLen int
	switch header[3] {
	case 0x01:
		addrLen = 4
	case 0x03:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return newError(ErrSocksDialFailed, opClient, "failed to read domain length", err)
		}
		addrLen = int(lenBuf[0])
	case 0x04:
		addrLen = 16
	default:
		return newError(ErrSocksDialFailed, opClient, "unknown address type in reply", nil)
	}
	if addrLen > 0 {
		if _, err := io.CopyN(io.Discard, conn, int64(addrLen)); err != nil {
			return newError(ErrSocksDialFailed, opClient, "failed to discard address bytes", err)
		}
	}
	if _, err := io.CopyN(io.Discard, conn, 2); err != nil {
		return newError(ErrSocksDialFailed, opClient, "failed to discard port bytes", err)
	}
	return nil
}
