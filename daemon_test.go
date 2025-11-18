package tornago

import (
	"bytes"
	"strings"
	"testing"
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
		p := &TorProcess{dataDir: "/tmp/tor"}
		if p.DataDir() != "/tmp/tor" {
			t.Errorf("expected DataDir /tmp/tor, got %s", p.DataDir())
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
