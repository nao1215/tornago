package tornago

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

// newReplyReader returns a ControlClient whose reply side reads data and whose
// command side discards writes, so the reply parser runs without a socket.
func newReplyReader(data string) *ControlClient {
	return &ControlClient{
		rw: bufio.NewReadWriter(bufio.NewReader(strings.NewReader(data)), bufio.NewWriter(io.Discard)),
	}
}

// stripLineBreaks removes CR and LF so a fuzzed string fits in one reply line.
func stripLineBreaks(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// FuzzReadReply feeds arbitrary bytes to the control reply parser, which is
// what a ControlPort (or anything listening on the configured address) sends back.
// The parser must not panic, and a successful parse must have seen a final
// "250 " line and must return no more lines than the input holds.
func FuzzReadReply(f *testing.F) {
	for _, seed := range []string{
		"250 OK\r\n",
		"250-First line\r\n250-Second line\r\n250 OK\r\n",
		"250+onion-address\r\ndata line 1\r\ndata line 2\r\n.\r\n250 OK\r\n",
		"250-ServiceID=abc\r\n250-PrivateKey=ED25519-V3:key\r\n250 OK\r\n",
		"250 SocksPort=9050\r\n",
		"552 Unrecognized command\r\n",
		"650 CIRC 1 BUILT\r\n250 OK\r\n",
		"451 Resource exhausted\r\n",
		"250+config-text=\r\n..hidden\r\n.\r\n250 OK\r\n",
		"25",
		"",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		lines, err := newReplyReader(string(data)).readReply()
		if err != nil {
			return
		}
		if !strings.Contains("\n"+string(data), "\n250 ") {
			t.Fatalf("reply accepted without a final 250 line: %q", data)
		}
		if got, limit := len(lines), strings.Count(string(data), "\n"); got > limit {
			t.Fatalf("parsed %d lines from input with %d line breaks", got, limit)
		}
		for _, line := range lines {
			if strings.ContainsAny(line, "\n") {
				t.Fatalf("parsed line %q still contains a line break", line)
			}
		}
	})
}

// FuzzReadReplyRoundTrip encodes a reply the way control-spec section 2.3
// describes (a mid reply line, a data reply with dot-stuffed lines, an
// asynchronous event interleaved before the end, and the final "250 OK") and
// checks that readReply gives back exactly the lines that were encoded.
func FuzzReadReplyRoundTrip(f *testing.F) {
	f.Add("ServiceID=abc", "config-text", "line 1\nline 2", "CIRC 1 BUILT")
	f.Add("", "onions/current", ".hidden\n..double\n.", "STREAM 1 NEW")
	f.Add("version=0.4.8", "k", "", "")
	f.Fuzz(func(t *testing.T, mid, key, block, event string) {
		mid = stripLineBreaks(mid)
		key = stripLineBreaks(key)
		event = stripLineBreaks(event)
		blockLines := strings.Split(strings.ReplaceAll(block, "\r", ""), "\n")

		var b strings.Builder
		b.WriteString("650 " + event + "\r\n")
		b.WriteString("250-" + mid + "\r\n")
		b.WriteString("250+" + key + "=\r\n")
		for _, line := range blockLines {
			if strings.HasPrefix(line, ".") {
				line = "." + line
			}
			b.WriteString(line + "\r\n")
		}
		b.WriteString(".\r\n")
		b.WriteString("250 OK\r\n")

		got, err := newReplyReader(b.String()).readReply()
		if err != nil {
			t.Fatalf("readReply(%q) failed: %v", b.String(), err)
		}
		want := append([]string{mid, key + "="}, blockLines...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("readReply(%q)\n got %q\nwant %q", b.String(), got, want)
		}
	})
}

// decodeQuotedString is the inverse of quotedString: it reads a control
// protocol QuotedString and reports whether the closing quote is the last byte.
func decodeQuotedString(s string) (string, bool) {
	if len(s) < 2 || s[0] != '"' {
		return "", false
	}
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
			if i >= len(s) {
				return "", false
			}
			b.WriteByte(s[i])
		case '"':
			return b.String(), i == len(s)-1
		default:
			b.WriteByte(s[i])
		}
	}
	return "", false
}

// FuzzQuotedString checks that quotedString always yields one QuotedString
// token that decodes back to its input.
func FuzzQuotedString(f *testing.F) {
	for _, seed := range []string{"password123", `path\to\file`, `my"password"`, `C:\path\to\"file"`, "", `\\\`, `"""`, "a b\tc"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		quoted := quotedString(s)
		got, ok := decodeQuotedString(quoted)
		if !ok {
			t.Fatalf("quotedString(%q) = %q is not a single QuotedString", s, quoted)
		}
		if got != s {
			t.Fatalf("quotedString(%q) = %q decodes to %q", s, quoted, got)
		}
	})
}

// FuzzSetConfSendsOneLine drives SetConf over an in-memory pipe and checks
// that whatever key and value the caller passes, Tor receives at most one
// command line: a key or value with a line break must be rejected before
// anything is written, because the rest of it would run as a second command.
func FuzzSetConfSendsOneLine(f *testing.F) {
	f.Add("MaxCircuitDirtiness", "600")
	f.Add("ExcludeNodes", "$AAAA,$BBBB")
	f.Add("Nickname", `with "quotes" and \ backslash`)
	f.Add("Nickname", "x\r\nSIGNAL SHUTDOWN")
	f.Add("Nickname\nSIGNAL HALT", "x")
	f.Fuzz(func(t *testing.T, key, value string) {
		if key == "" {
			return
		}
		clientConn, serverConn := net.Pipe()
		type received struct {
			first string
			rest  string
		}
		done := make(chan received, 1)
		go func() {
			br := bufio.NewReader(serverConn)
			first, err := br.ReadString('\n')
			if err == nil {
				go func() {
					_, _ = serverConn.Write([]byte("250 OK\r\n")) //nolint:errcheck // the client may already be gone
				}()
			}
			rest, _ := io.ReadAll(br) //nolint:errcheck // EOF once the client closes
			_ = serverConn.Close()
			done <- received{first: first, rest: string(rest)}
		}()

		client := &ControlClient{
			conn:          clientConn,
			rw:            bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn)),
			timeout:       5 * time.Second,
			authenticated: true,
		}
		err := client.SetConf(context.Background(), key, value)
		_ = client.Close()
		got := <-done

		if strings.ContainsAny(key+value, "\r\n") {
			if !errors.Is(err, &TornagoError{Kind: ErrInvalidConfig}) {
				t.Fatalf("SetConf(%q, %q) error = %v, want ErrInvalidConfig", key, value, err)
			}
			if got.first != "" || got.rest != "" {
				t.Fatalf("SetConf(%q, %q) wrote %q%q to the ControlPort", key, value, got.first, got.rest)
			}
			return
		}
		if err != nil {
			t.Fatalf("SetConf(%q, %q) failed: %v", key, value, err)
		}
		want := "SETCONF " + key + "=" + quotedString(value) + "\r\n"
		if got.first != want || got.rest != "" {
			t.Fatalf("SetConf(%q, %q) wrote %q%q, want %q", key, value, got.first, got.rest, want)
		}
	})
}

// FuzzParseStatusLines checks the circuit-status and stream-status line
// parsers: they never panic, the leading fields land in ID/Status, and the
// amount of whitespace between fields does not change the result.
func FuzzParseStatusLines(f *testing.F) {
	for _, seed := range []string{
		"1 BUILT $AAAA~relay1,$BBBB~relay2 BUILD_FLAGS=NEED_CAPACITY PURPOSE=GENERAL TIME_CREATED=2024-01-01T00:00:00",
		"2 EXTENDED PURPOSE=HS_CLIENT_INTRO",
		"3 SUCCEEDED 1 example.com:80 PURPOSE=USER",
		"1",
		"",
		"  4\tNEW  0   host:443\t",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		fields := strings.Fields(line)
		normalized := strings.Join(fields, " ")

		circuit := parseCircuitLine(line)
		if len(fields) < 2 {
			if !reflect.DeepEqual(circuit, CircuitInfo{}) {
				t.Fatalf("parseCircuitLine(%q) = %+v, want zero value", line, circuit)
			}
		} else if circuit.ID != fields[0] || circuit.Status != fields[1] {
			t.Fatalf("parseCircuitLine(%q) = %+v, want ID %q Status %q", line, circuit, fields[0], fields[1])
		}
		if again := parseCircuitLine(normalized); !reflect.DeepEqual(circuit, again) {
			t.Fatalf("parseCircuitLine differs on whitespace: %+v vs %+v", circuit, again)
		}

		stream := parseStreamLine(line)
		if len(fields) < 4 {
			if !reflect.DeepEqual(stream, StreamInfo{}) {
				t.Fatalf("parseStreamLine(%q) = %+v, want zero value", line, stream)
			}
		} else if stream.ID != fields[0] || stream.Status != fields[1] || stream.CircuitID != fields[2] || stream.Target != fields[3] {
			t.Fatalf("parseStreamLine(%q) = %+v, fields %q", line, stream, fields)
		}
		if again := parseStreamLine(normalized); !reflect.DeepEqual(stream, again) {
			t.Fatalf("parseStreamLine differs on whitespace: %+v vs %+v", stream, again)
		}
	})
}

// readerConn is a net.Conn whose reads come from r. Only Read is used by the
// SOCKS5 reply reader; any other method would panic on the nil embedded Conn.
type readerConn struct {
	net.Conn
	r io.Reader
}

func (c readerConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// FuzzConsumeConnectReply feeds arbitrary bytes to the SOCKS5 CONNECT reply
// reader, which is what the SOCKS port answers before the tunneled stream
// starts. It must accept exactly the replies RFC 1928 section 6 describes
// with REP=0 (the VER byte is not checked), and on success it must leave every
// byte after the reply unread: those bytes belong to the tunneled stream.
func FuzzConsumeConnectReply(f *testing.F) {
	for _, seed := range [][]byte{
		{5, 0, 0, 1, 127, 0, 0, 1, 0, 80},
		{5, 0, 0, 3, 3, 'a', 'b', 'c', 1, 187, 'H', 'T', 'T', 'P'},
		{5, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 22},
		{5, 1, 0, 1, 0, 0, 0, 0, 0, 0},
		{5, 0, 0, 9},
		{5, 0, 0, 3, 0, 0, 0},
		{5, 0},
		{},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// The reference length of a valid reply, or -1.
		want := -1
		if len(data) >= 4 && data[1] == 0 {
			switch data[3] {
			case 1:
				want = 4 + 4 + 2
			case 4:
				want = 4 + 16 + 2
			case 3:
				if len(data) >= 5 {
					want = 4 + 1 + int(data[4]) + 2
				}
			}
		}
		if want > len(data) {
			want = -1
		}

		r := bytes.NewReader(data)
		err := consumeConnectReply(readerConn{r: r})
		if (err == nil) != (want >= 0) {
			t.Fatalf("consumeConnectReply(%x) error = %v, want ok=%v", data, err, want >= 0)
		}
		if err == nil && r.Len() != len(data)-want {
			t.Fatalf("consumeConnectReply(%x) left %d bytes, want %d", data, r.Len(), len(data)-want)
		}
		var te *TornagoError
		if err != nil && (!errors.As(err, &te) || te.Kind != ErrSocksDialFailed) {
			t.Fatalf("consumeConnectReply(%x) error = %v, want ErrSocksDialFailed", data, err)
		}
	})
}

// FuzzConnectRequestRoundTrip builds a SOCKS5 CONNECT request for an
// arbitrary host and port and decodes it with the RFC 1928 decoder of
// property_test.go: a host the request accepts must come back unchanged (an
// IP address as the same address), and one it refuses must be an empty or
// over-long domain name.
func FuzzConnectRequestRoundTrip(f *testing.F) {
	f.Add("example.com", uint16(443))
	f.Add("127.0.0.1", uint16(80))
	f.Add("::1", uint16(22))
	f.Add("::ffff:10.0.0.1", uint16(8080))
	f.Add("fe80::1%eth0", uint16(1))
	f.Add("", uint16(0))
	f.Add(strings.Repeat("a", 256), uint16(65535))
	f.Fuzz(func(t *testing.T, host string, port uint16) {
		req, err := buildConnectRequest(host, port)
		isIP := net.ParseIP(host) != nil
		if err != nil {
			if isIP || (len(host) != 0 && len(host) <= 255) {
				t.Fatalf("buildConnectRequest(%q, %d) refused a valid host: %v", host, port, err)
			}
			return
		}
		gotHost, gotPort, err := decodeConnectRequest(req)
		if err != nil {
			t.Fatalf("buildConnectRequest(%q, %d) = %x, which does not decode: %v", host, port, req, err)
		}
		if gotPort != port {
			t.Fatalf("port %d came back as %d", port, gotPort)
		}
		if isIP {
			if !net.ParseIP(host).Equal(net.ParseIP(gotHost)) {
				t.Fatalf("address %q came back as %q", host, gotHost)
			}
			return
		}
		if gotHost != host {
			t.Fatalf("domain %q came back as %q", host, gotHost)
		}
	})
}
