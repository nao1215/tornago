package tornago

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestQuotedString(t *testing.T) {
	t.Run("should quote simple string without special characters", func(t *testing.T) {
		result := quotedString("password123")
		expected := `"password123"`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("should escape backslash characters in string", func(t *testing.T) {
		result := quotedString(`path\to\file`)
		expected := `"path\\to\\file"`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("should escape double quote characters in string", func(t *testing.T) {
		result := quotedString(`my"password"`)
		expected := `"my\"password\""`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("should escape both backslash and double quote characters", func(t *testing.T) {
		result := quotedString(`C:\path\to\"file"`)
		expected := `"C:\\path\\to\\\"file\""`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("should handle empty string by returning empty quoted string", func(t *testing.T) {
		result := quotedString("")
		expected := `""`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("should correctly quote string with only backslashes", func(t *testing.T) {
		result := quotedString(`\\\`)
		expected := `"\\\\\\"`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})

	t.Run("should correctly quote string with only double quotes", func(t *testing.T) {
		result := quotedString(`"""`)
		expected := `"\"\"\""`
		if result != expected {
			t.Fatalf("expected %s, got %s", expected, result)
		}
	})
}

func TestTryGetCookiePath(t *testing.T) {
	t.Run("should parse cookie path from PROTOCOLINFO response", func(t *testing.T) {
		// Create a mock control port server
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		addr := listener.Addr().String()

		// Handle connection in background
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Read PROTOCOLINFO command
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil {
				return
			}

			// Send mock PROTOCOLINFO response with cookie path
			response := "250-PROTOCOLINFO 1\r\n"
			response += "250-AUTH METHODS=COOKIE,SAFECOOKIE COOKIEFILE=\"/tmp/test-cookie\"\r\n"
			response += "250-VERSION Tor=\"0.4.8.0\"\r\n"
			response += "250 OK\r\n"
			_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
		}()

		// Small delay to ensure server is ready
		time.Sleep(10 * time.Millisecond)

		cookiePath, err := tryGetCookiePath(addr)
		if err != nil {
			t.Fatalf("tryGetCookiePath failed: %v", err)
		}

		if !strings.Contains(cookiePath, "test-cookie") {
			t.Errorf("expected cookie path to contain 'test-cookie', got %s", cookiePath)
		}
	})

	t.Run("should return error when COOKIEFILE missing", func(t *testing.T) {
		// Create a mock control port server
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		addr := listener.Addr().String()

		// Handle connection in background
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Read PROTOCOLINFO command
			buf := make([]byte, 1024)
			_, err = conn.Read(buf)
			if err != nil {
				return
			}

			// Send mock PROTOCOLINFO response without cookie path
			response := "250-PROTOCOLINFO 1\r\n"
			response += "250-AUTH METHODS=NULL\r\n"
			response += "250-VERSION Tor=\"0.4.8.0\"\r\n"
			response += "250 OK\r\n"
			_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
		}()

		// Small delay to ensure server is ready
		time.Sleep(10 * time.Millisecond)

		_, err = tryGetCookiePath(addr)
		if err == nil {
			t.Error("expected error when COOKIEFILE is missing")
		}
		if !strings.Contains(err.Error(), "COOKIEFILE missing") {
			t.Errorf("expected error about missing COOKIEFILE, got: %v", err)
		}
	})

	t.Run("should return error when connection fails", func(t *testing.T) {
		// Try to connect to non-existent port
		_, err := tryGetCookiePath("127.0.0.1:1")
		if err == nil {
			t.Error("expected error when connection fails")
		}
	})
}

func TestWaitForControlPort(t *testing.T) {
	t.Run("should succeed when control port is ready", func(t *testing.T) {
		// Create a mock control port server
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		addr := listener.Addr().String()

		// Create a temporary cookie file
		tmpFile := fmt.Sprintf("/tmp/tornago-test-cookie-%d", time.Now().UnixNano())
		err = createTempCookieFile(tmpFile)
		if err != nil {
			t.Fatalf("failed to create temp cookie: %v", err)
		}
		defer removeTempFile(tmpFile)

		// Handle connections in background
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go func(c net.Conn) {
					defer c.Close()

					// Read PROTOCOLINFO command
					buf := make([]byte, 1024)
					_, err := c.Read(buf)
					if err != nil {
						return
					}

					// Send mock PROTOCOLINFO response with cookie path
					response := "250-PROTOCOLINFO 1\r\n"
					response += fmt.Sprintf("250-AUTH METHODS=COOKIE COOKIEFILE=\"%s\"\r\n", tmpFile)
					response += "250-VERSION Tor=\"0.4.8.0\"\r\n"
					response += "250 OK\r\n"
					_, _ = c.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
				}(conn)
			}
		}()

		// Small delay to ensure server is ready
		time.Sleep(10 * time.Millisecond)

		err = WaitForControlPort(addr, 5*time.Second)
		if err != nil {
			t.Errorf("WaitForControlPort failed: %v", err)
		}
	})

	t.Run("should timeout when control port not ready", func(t *testing.T) {
		// Try to wait for non-existent port with short timeout
		err := WaitForControlPort("127.0.0.1:1", 100*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("expected timeout error, got: %v", err)
		}
	})

	t.Run("should retry until cookie file exists", func(t *testing.T) {
		// Create a mock control port server
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		addr := listener.Addr().String()

		// Create a temporary cookie file path (but don't create the file yet)
		tmpFile := fmt.Sprintf("/tmp/tornago-test-cookie-%d", time.Now().UnixNano())
		defer removeTempFile(tmpFile)

		// Handle connections in background
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}

				go func(c net.Conn) {
					defer c.Close()

					// Read PROTOCOLINFO command
					buf := make([]byte, 1024)
					_, err := c.Read(buf)
					if err != nil {
						return
					}

					// Send mock PROTOCOLINFO response with cookie path
					response := "250-PROTOCOLINFO 1\r\n"
					response += fmt.Sprintf("250-AUTH METHODS=COOKIE COOKIEFILE=\"%s\"\r\n", tmpFile)
					response += "250-VERSION Tor=\"0.4.8.0\"\r\n"
					response += "250 OK\r\n"
					_, _ = c.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
				}(conn)
			}
		}()

		// Create the cookie file after a delay
		go func() {
			time.Sleep(500 * time.Millisecond)
			_ = createTempCookieFile(tmpFile) //nolint:errcheck
		}()

		// Small delay to ensure server is ready
		time.Sleep(10 * time.Millisecond)

		// Should wait and succeed once cookie file is created
		err = WaitForControlPort(addr, 3*time.Second)
		if err != nil {
			t.Errorf("WaitForControlPort failed: %v", err)
		}
	})
}

// Helper function to create a temporary cookie file
func createTempCookieFile(path string) error {
	// Write some dummy cookie data
	cookie := []byte("dummy-cookie-data-for-testing")
	return os.WriteFile(path, cookie, 0600)
}

// Helper function to remove a temporary file
func removeTempFile(path string) {
	_ = os.Remove(path) //nolint:errcheck
}

func TestAuthToken(t *testing.T) {
	t.Run("should generate token from password", func(t *testing.T) {
		auth := ControlAuthFromPassword("test-password")
		client := &ControlClient{
			auth: auth,
		}

		token, err := client.authToken()
		if err != nil {
			t.Fatalf("authToken failed: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token for password auth")
		}
		// Password should be quoted
		if token[0] != '"' || token[len(token)-1] != '"' {
			t.Errorf("expected quoted token, got: %s", token)
		}
	})

	t.Run("should generate token from cookie bytes", func(t *testing.T) {
		cookieData := []byte("test-cookie-bytes-for-auth")
		auth := ControlAuth{cookieBytes: cookieData}
		client := &ControlClient{
			auth: auth,
		}

		token, err := client.authToken()
		if err != nil {
			t.Fatalf("authToken failed: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token for cookie bytes auth")
		}
		// Should be uppercase hex
		for _, ch := range token {
			if (ch < '0' || ch > '9') && (ch < 'A' || ch > 'F') {
				t.Errorf("expected uppercase hex token, got: %s", token)
				break
			}
		}
	})

	t.Run("should generate token from cookie file", func(t *testing.T) {
		// Create temporary cookie file
		tmpFile := "/tmp/tornago-test-auth-cookie"
		cookieData := []byte("test-file-cookie-content")
		err := os.WriteFile(tmpFile, cookieData, 0600)
		if err != nil {
			t.Fatalf("failed to create test cookie file: %v", err)
		}
		defer os.Remove(tmpFile)

		auth := ControlAuth{cookiePath: tmpFile}
		client := &ControlClient{
			auth: auth,
		}

		token, err := client.authToken()
		if err != nil {
			t.Fatalf("authToken failed: %v", err)
		}
		if token == "" {
			t.Error("expected non-empty token for cookie file auth")
		}
		// Should be uppercase hex
		for _, ch := range token {
			if (ch < '0' || ch > '9') && (ch < 'A' || ch > 'F') {
				t.Errorf("expected uppercase hex token, got: %s", token)
				break
			}
		}
	})

	t.Run("should return empty token when no auth configured", func(t *testing.T) {
		auth := ControlAuth{}
		client := &ControlClient{
			auth: auth,
		}

		token, err := client.authToken()
		if err != nil {
			t.Fatalf("authToken failed: %v", err)
		}
		if token != "" {
			t.Errorf("expected empty token for no auth, got: %s", token)
		}
	})

	t.Run("should fail when cookie file does not exist", func(t *testing.T) {
		auth := ControlAuth{cookiePath: "/nonexistent/cookie/path"}
		client := &ControlClient{
			auth: auth,
		}

		_, err := client.authToken()
		if err == nil {
			t.Error("expected authToken to fail with nonexistent cookie file")
		}
	})
}

func TestControlClientAdditional(t *testing.T) {
	t.Run("should close control client successfully", func(t *testing.T) {
		// Create a mock control server
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
			// Keep connection open
			time.Sleep(1 * time.Second)
			_ = conn.Close()
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			5*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}

		// Test Close
		err = client.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}

		// Close again should not error (conn is already nil)
		// Note: the implementation should handle this gracefully
		_ = client.Close() // Don't check error - second close may fail
	})
}

func TestControlAuthFromTor(t *testing.T) {
	t.Run("should retrieve auth from running Tor", func(t *testing.T) {
		// Create a temporary cookie file
		tmpFile := "/tmp/tornago-test-control-cookie"
		cookieData := []byte("test-cookie-data-12345678901234567890123456789012")
		err := os.WriteFile(tmpFile, cookieData, 0600)
		if err != nil {
			t.Fatalf("failed to create cookie file: %v", err)
		}
		defer os.Remove(tmpFile)

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
			defer conn.Close()

			buf := make([]byte, 1024)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					return
				}

				command := string(buf[:n])
				// Handle PROTOCOLINFO
				if strings.Contains(command, "PROTOCOLINFO") {
					response := "250-PROTOCOLINFO 1\r\n"
					response += "250-AUTH METHODS=COOKIE,SAFECOOKIE COOKIEFILE=\"" + tmpFile + "\"\r\n"
					response += "250-VERSION Tor=\"0.4.8.0\"\r\n"
					response += "250 OK\r\n"
					_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
					continue
				}
				// Handle AUTHENTICATE (ControlAuthFromTor will authenticate)
				if strings.Contains(command, "AUTHENTICATE") {
					_, _ = conn.Write([]byte("250 OK\r\n")) //nolint:errcheck
					return
				}
			}
		}()

		// Small delay to ensure server is ready
		time.Sleep(10 * time.Millisecond)

		auth, cookiePath, err := ControlAuthFromTor(listener.Addr().String(), 2*time.Second)
		if err != nil {
			t.Fatalf("ControlAuthFromTor failed: %v", err)
		}

		if cookiePath == "" {
			t.Error("expected non-empty cookie path")
		}

		if len(auth.cookieBytes) == 0 {
			t.Error("expected non-empty cookie bytes")
		}
	})

	t.Run("should fail when PROTOCOLINFO fails", func(t *testing.T) {
		// Try to connect to non-existent port
		_, _, err := ControlAuthFromTor("127.0.0.1:1", 100*time.Millisecond)
		if err == nil {
			t.Error("expected ControlAuthFromTor to fail with non-existent port")
		}
	})
}

func TestNewIdentity(t *testing.T) {
	t.Run("should send NEWNYM signal", func(t *testing.T) {
		// Create mock control server
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		receivedCommand := make(chan string, 1)

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
				// First handle AUTHENTICATE
				if strings.Contains(command, "AUTHENTICATE") {
					_, _ = conn.Write([]byte("250 OK\r\n")) //nolint:errcheck
					continue
				}
				// Then handle SIGNAL NEWNYM
				if strings.Contains(command, "SIGNAL") {
					receivedCommand <- command
					_, _ = conn.Write([]byte("250 OK\r\n")) //nolint:errcheck
					return
				}
			}
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			2*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		err = client.NewIdentity(ctx)
		if err != nil {
			t.Fatalf("NewIdentity failed: %v", err)
		}

		select {
		case cmd := <-receivedCommand:
			if !strings.Contains(cmd, "NEWNYM") {
				t.Errorf("expected NEWNYM signal, got: %s", cmd)
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for SIGNAL command")
		}
	})

	t.Run("should fail with invalid connection", func(t *testing.T) {
		client, err := NewControlClient(
			"127.0.0.1:1",
			ControlAuth{},
			100*time.Millisecond,
		)
		if err != nil {
			t.Skip("failed to create client, which is expected")
		}
		defer client.Close()

		ctx := context.Background()
		err = client.NewIdentity(ctx)
		if err == nil {
			t.Error("expected NewIdentity to fail with invalid connection")
		}
	})
}

func TestGetInfoNoAuth(t *testing.T) {
	t.Run("should get info without authentication", func(t *testing.T) {
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
			defer conn.Close()

			buf := make([]byte, 1024)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					return
				}

				command := string(buf[:n])
				if strings.Contains(command, "GETINFO") {
					// Send version info
					response := "250-version=0.4.8.0\r\n250 OK\r\n"
					_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
					return
				}
			}
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			2*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		info, err := client.GetInfoNoAuth(ctx, "version")
		if err != nil {
			t.Fatalf("GetInfoNoAuth failed: %v", err)
		}

		if !strings.Contains(info, "0.4.8.0") {
			t.Errorf("expected version info, got: %s", info)
		}
	})
}

func TestReadReply(t *testing.T) {
	t.Run("should handle multi-line responses", func(t *testing.T) {
		// Create a mock control server that sends multi-line response
		lc := net.ListenConfig{}
		listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create listener: %v", err)
		}
		defer listener.Close()

		done := make(chan struct{})
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

				// Send multi-line response for any other command
				response := "250-First line\r\n"
				response += "250-Second line\r\n"
				response += "250 OK\r\n"
				_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
				// Wait before closing to ensure client reads everything
				time.Sleep(100 * time.Millisecond)
				close(done)
				return
			}
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			2*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}
		defer client.Close()

		// First authenticate
		err = client.Authenticate()
		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}

		ctx := context.Background()
		lines, err := client.execCommand(ctx, "TEST")
		if err != nil {
			t.Fatalf("execCommand failed: %v", err)
		}

		if len(lines) < 2 {
			t.Errorf("expected at least 2 lines, got %d", len(lines))
		}

		<-done
	})

	t.Run("should handle error responses", func(t *testing.T) {
		// Create a mock control server that sends error
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
			_, _ = conn.Read(buf) //nolint:errcheck // Test mock server, error doesn't affect test outcome // Read any command

			// Send error response
			response := "552 Unrecognized command\r\n"
			_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			2*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		_, err = client.execCommand(ctx, "INVALID")
		if err == nil {
			t.Error("expected execCommand to fail with error response")
		}
	})
}

func TestReadDataBlock(t *testing.T) {
	t.Run("should handle 250+ data block responses", func(t *testing.T) {
		// Create mock control server that sends 250+ data block
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

				// Send 250+ data block response
				response := "250+onion-address\r\n"
				response += "data line 1\r\n"
				response += "data line 2\r\n"
				response += "data line 3\r\n"
				response += ".\r\n"
				response += "250 OK\r\n"
				_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
				time.Sleep(100 * time.Millisecond)
				return
			}
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			2*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}
		defer client.Close()

		// Authenticate first
		err = client.Authenticate()
		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}

		ctx := context.Background()
		lines, err := client.execCommand(ctx, "GETINFO")
		if err != nil {
			t.Fatalf("execCommand failed: %v", err)
		}

		// Should include the data block lines
		if len(lines) < 3 {
			t.Errorf("expected at least 3 lines from data block, got %d", len(lines))
		}
	})
}

func TestGetInfoError(t *testing.T) {
	t.Run("should return error when key not found in response", func(t *testing.T) {
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
				if strings.Contains(command, "GETINFO") {
					// Send response without the requested key
					response := "250-other-key=value\r\n250 OK\r\n"
					_, _ = conn.Write([]byte(response)) //nolint:errcheck // Test mock server, error doesn't affect test outcome
					return
				}
			}
		}()

		client, err := NewControlClient(
			listener.Addr().String(),
			ControlAuth{},
			2*time.Second,
		)
		if err != nil {
			t.Fatalf("failed to create control client: %v", err)
		}
		defer client.Close()

		ctx := context.Background()
		_, err = client.GetInfo(ctx, "requested-key")
		if err == nil {
			t.Error("expected error when key not found in response")
		}
	})
}
