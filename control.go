package tornago

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// opControlClient labels errors originating from ControlClient operations.
	opControlClient = "ControlClient"
)

// ControlClient talks to Tor's ControlPort (a text-based management interface
// where Tor accepts commands like AUTHENTICATE/GETINFO/SIGNAL NEWNYM). It is
// provided as a standalone client so tools that only need ControlPort access
// (e.g. circuit rotation or Hidden Service management) can use it without
// constructing the higher-level HTTP/TCP Client.
//
// The ControlPort allows you to:
//   - Rotate circuits to get new exit IPs (NewIdentity)
//   - Create and manage hidden services (CreateHiddenService)
//   - Query Tor's internal state (GetInfo)
//   - Monitor Tor events and status
//
// Authentication is required before most commands. Use either cookie-based
// authentication (automatic with StartTorDaemon) or password authentication
// (for existing Tor instances).
//
// Example usage:
//
//	auth := tornago.ControlAuthFromCookie("/var/lib/tor/control_auth_cookie")
//	ctrl, _ := tornago.NewControlClient("127.0.0.1:9051", auth, 5*time.Second)
//	defer ctrl.Close()
//
//	ctrl.Authenticate()
//	ctrl.NewIdentity(context.Background())  // Request new circuits
type ControlClient struct {
	// conn is the underlying TCP connection to the ControlPort.
	conn net.Conn
	// rw buffers reads/writes for the control protocol.
	rw *bufio.ReadWriter
	// timeout bounds network operations for each command.
	timeout time.Duration
	// auth contains authentication material for ControlPort access.
	auth ControlAuth
	// authenticated reports whether AUTHENTICATE succeeded.
	authenticated bool
	// mu serializes command writes/reads.
	mu sync.Mutex
}

// NewControlClient dials the ControlPort at addr with the given timeout.
func NewControlClient(addr string, auth ControlAuth, timeout time.Duration) (*ControlClient, error) {
	if addr == "" {
		return nil, newError(ErrInvalidConfig, opControlClient, "ControlAddr is empty", nil)
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, newError(ErrControlRequestFail, opControlClient, "failed to dial ControlPort", err)
	}

	client := &ControlClient{
		conn:    conn,
		rw:      bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
		timeout: timeout,
		auth:    auth,
	}
	return client, nil
}

// Authenticate performs AUTHENTICATE using ControlAuth credentials.
func (c *ControlClient) Authenticate() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	token, err := c.authToken()
	if err != nil {
		return err
	}
	cmd := "AUTHENTICATE"
	if token != "" {
		cmd = "AUTHENTICATE " + token
	}
	if _, err := c.execCommand(ctx, cmd); err != nil {
		return err
	}
	c.authenticated = true
	return nil
}

// NewIdentity issues SIGNAL NEWNYM to rotate Tor circuits, causing Tor to
// close existing circuits and build new ones. This effectively gives you a
// new exit IP address for subsequent requests.
//
// This is useful for:
//   - Avoiding rate limiting or IP-based blocks
//   - Getting a fresh identity for privacy reasons
//   - Testing behavior with different exit nodes
//
// Note: Tor rate-limits NEWNYM requests to once per 10 seconds by default.
// Calling this more frequently will not create new circuits.
func (c *ControlClient) NewIdentity(ctx context.Context) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}
	_, err := c.execCommand(ctx, "SIGNAL NEWNYM")
	return err
}

// GetInfo runs GETINFO and returns the associated value.
func (c *ControlClient) GetInfo(ctx context.Context, key string) (string, error) {
	return c.getInfo(ctx, key, true)
}

// GetInfoNoAuth runs GETINFO without authenticating first.
func (c *ControlClient) GetInfoNoAuth(ctx context.Context, key string) (string, error) {
	return c.getInfo(ctx, key, false)
}

func (c *ControlClient) getInfo(ctx context.Context, key string, requireAuth bool) (string, error) {
	if key == "" {
		return "", newError(ErrInvalidConfig, opControlClient, "GetInfo key is empty", nil)
	}
	if requireAuth {
		if err := c.ensureAuthenticated(); err != nil {
			return "", err
		}
	}
	lines, err := c.execCommand(ctx, "GETINFO "+key)
	if err != nil {
		return "", err
	}
	prefix := key + "="
	var result string
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			result = strings.TrimPrefix(line, prefix)
		}
	}
	if result == "" {
		return "", newError(ErrControlRequestFail, opControlClient, "key not found in GETINFO response", nil)
	}
	return result, nil
}

// Close closes the underlying ControlPort connection.
func (c *ControlClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// ensureAuthenticated runs Authenticate if it has not been performed yet.
func (c *ControlClient) ensureAuthenticated() error {
	if c.authenticated {
		return nil
	}
	return c.Authenticate()
}

// authToken derives the authentication token based on ControlAuth settings.
func (c *ControlClient) authToken() (string, error) {
	switch {
	case c.auth.Password() != "":
		return quotedString(c.auth.Password()), nil
	case c.auth.CookiePath() != "":
		path := filepath.Clean(c.auth.CookiePath())
		data, err := os.ReadFile(path)
		if err != nil {
			return "", newError(ErrIO, opControlClient, "failed to read control cookie", err)
		}
		return strings.ToUpper(hex.EncodeToString(data)), nil
	case len(c.auth.CookieBytes()) != 0:
		return strings.ToUpper(hex.EncodeToString(c.auth.CookieBytes())), nil
	default:
		return "", nil
	}
}

// execCommand sends a control command and returns the response lines.
func (c *ControlClient) execCommand(ctx context.Context, cmd string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.applyDeadline(ctx); err != nil {
		return nil, err
	}
	defer c.clearDeadline()

	if _, err := c.rw.WriteString(cmd + "\r\n"); err != nil {
		return nil, newError(ErrControlRequestFail, opControlClient, "failed to write command", err)
	}
	if err := c.rw.Flush(); err != nil {
		return nil, newError(ErrControlRequestFail, opControlClient, "failed to flush command", err)
	}
	return c.readReply()
}

// ControlAuthFromTor queries Tor for the control cookie path and returns the
// ControlAuth that uses the corresponding cookie bytes.
func ControlAuthFromTor(controlAddr string, timeout time.Duration) (ControlAuth, string, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		client, err := NewControlClient(controlAddr, ControlAuth{}, 5*time.Second)
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lines, err := client.execCommand(ctx, "PROTOCOLINFO 1")
		cancel()
		if err != nil {
			lastErr = err
			_ = client.Close()
			time.Sleep(300 * time.Millisecond)
			continue
		}

		var cookiePath string
		for _, line := range lines {
			if idx := strings.Index(line, `COOKIEFILE="`); idx >= 0 {
				start := idx + len(`COOKIEFILE="`)
				end := strings.Index(line[start:], `"`)
				if end >= 0 {
					cookiePath = filepath.Clean(line[start : start+end])
					break
				}
			}
		}
		if cookiePath == "" {
			lastErr = errors.New("control-port-file missing from PROTOCOLINFO")
			_ = client.Close()
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// #nosec G304 -- path comes from Tor control protocol and is sanitized by Tor itself.
		data, err := os.ReadFile(cookiePath)
		if err != nil {
			lastErr = err
			_ = client.Close()
			time.Sleep(300 * time.Millisecond)
			continue
		}

		hexCookie := strings.ToUpper(hex.EncodeToString(data))
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		_, err = client.execCommand(ctx, "AUTHENTICATE "+hexCookie)
		cancel()
		_ = client.Close()

		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}

		return ControlAuthFromCookieBytes(data), cookiePath, nil
	}

	if lastErr == nil {
		lastErr = errors.New("timed out waiting for control authentication")
	}
	return ControlAuth{}, "", newError(ErrControlRequestFail, opControlClient, "failed to authenticate control port", lastErr)
}

// applyDeadline sets connection deadlines derived from ctx and client timeout.
func (c *ControlClient) applyDeadline(ctx context.Context) error {
	if c.conn == nil {
		return newError(ErrControlRequestFail, opControlClient, "connection is closed", nil)
	}
	deadline, ok := ctx.Deadline()
	if c.timeout > 0 {
		t := time.Now().Add(c.timeout)
		if !ok || t.Before(deadline) {
			deadline = t
			ok = true
		}
	}
	if !ok {
		return c.conn.SetDeadline(time.Time{})
	}
	return c.conn.SetDeadline(deadline)
}

// clearDeadline removes any deadline on the underlying connection.
func (c *ControlClient) clearDeadline() {
	if c.conn != nil {
		//nolint:errcheck,gosec // best-effort reset to no deadline.
		c.conn.SetDeadline(time.Time{})
	}
}

// readReply parses the control response, handling data blocks and status codes.
func (c *ControlClient) readReply() ([]string, error) {
	var lines []string
	for {
		line, err := c.rw.ReadString('\n')
		if err != nil {
			return nil, newError(ErrControlRequestFail, opControlClient, "failed to read control response", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 3 {
			continue
		}
		codeStr := line[:3]
		code, convErr := strconv.Atoi(codeStr)
		if convErr != nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "250 "):
			if line == "250 OK" {
				return lines, nil
			}
			lines = append(lines, line[4:])
		case strings.HasPrefix(line, "250-"):
			lines = append(lines, line[4:])
		case strings.HasPrefix(line, "250+"):
			data, err := c.readDataBlock()
			if err != nil {
				return nil, err
			}
			lines = append(lines, line[4:])
			lines = append(lines, data...)
		case code >= 500:
			return nil, newError(ErrControlRequestFail, opControlClient, line, fmt.Errorf("%s", line))
		default:
			// Ignore asynchronous events (e.g. 650) for now.
		}
	}
}

// readDataBlock reads a 250+ data block until the terminating "." line.
func (c *ControlClient) readDataBlock() ([]string, error) {
	var block []string
	for {
		line, err := c.rw.ReadString('\n')
		if err != nil {
			return nil, newError(ErrControlRequestFail, opControlClient, "failed to read data block", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "." {
			return block, nil
		}
		block = append(block, line)
	}
}

// quotedString escapes special characters per control protocol expectations.
func quotedString(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, replacer.Replace(s))
}

// WaitForControlPort waits until Tor's control port is usable.
// Tor may accept TCP connections before it can respond to PROTOCOLINFO,
// because the cookie might not be created yet. This function verifies that
// PROTOCOLINFO succeeds AND the cookie file exists before returning.
func WaitForControlPort(controlAddr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// 1) Try to get cookie path without authenticating - this verifies PROTOCOLINFO works
		cookiePath, err := tryGetCookiePath(controlAddr)
		if err != nil {
			time.Sleep(1 * time.Second) // Increased from 300ms to avoid interfering with bootstrap
			continue
		}
		// 2) Verify cookie file exists and is non-empty
		if stat, err := os.Stat(cookiePath); err == nil && stat.Size() > 0 {
			// 3) Make one final verification that PROTOCOLINFO still works
			// (in case cookie was created but Tor is still initializing)
			if _, verifyErr := tryGetCookiePath(controlAddr); verifyErr == nil {
				return nil
			}
		}
		time.Sleep(1 * time.Second) // Increased from 300ms to avoid interfering with bootstrap
	}

	return fmt.Errorf("timed out waiting for control port %s to become usable", controlAddr)
}

func tryGetCookiePath(controlAddr string) (string, error) {
	client, err := NewControlClient(controlAddr, ControlAuth{}, 2*time.Second)
	if err != nil {
		return "", err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	lines, err := client.execCommand(ctx, "PROTOCOLINFO 1")
	cancel()
	if err != nil {
		return "", err
	}

	for _, line := range lines {
		if idx := strings.Index(line, `COOKIEFILE="`); idx >= 0 {
			start := idx + len(`COOKIEFILE="`)
			end := strings.Index(line[start:], `"`)
			if end >= 0 {
				return filepath.Clean(line[start : start+end]), nil
			}
		}
	}
	return "", errors.New("COOKIEFILE missing from PROTOCOLINFO response")
}
