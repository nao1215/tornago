package tornago

// Property, metamorphic and differential tests for the parts of tornago that
// compute something from their input without a Tor daemon: the SOCKS5 CONNECT
// request, the control-port status lines and ADD_ONION command, and the
// statistics behind metrics and slow-relay avoidance.
//
// The inputs are drawn with testing/quick from the standard library, so the
// module keeps no dependencies, not even test-only ones. `go test` draws 100
// inputs per property; the nightly Property workflow passes -quickchecks=20000.
// A failure prints the input that broke the property.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"net/netip"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// quickConfig returns the testing/quick configuration shared by the tests in
// this file. MaxCount stays 0 so that -quickchecks decides the number of inputs.
func quickConfig() *quick.Config {
	return &quick.Config{}
}

// checkProperty runs quick.Check and reports the failing input on error.
func checkProperty(t *testing.T, f any) {
	t.Helper()
	if err := quick.Check(f, quickConfig()); err != nil {
		var ce *quick.CheckError
		if errors.As(err, &ce) {
			t.Fatalf("property failed on input #%d: %#v", ce.Count, ce.In)
		}
		t.Fatal(err)
	}
}

// decodeConnectRequest decodes a SOCKS5 CONNECT request as RFC 1928 section 4
// lays it out. It is written from the RFC, apart from buildConnectRequest, so
// the two only agree when the request is encoded correctly.
func decodeConnectRequest(req []byte) (host string, port uint16, err error) {
	if len(req) < 4 {
		return "", 0, errors.New("request shorter than its header")
	}
	if req[0] != 5 || req[1] != 1 || req[2] != 0 {
		return "", 0, fmt.Errorf("header %x is not VER=5 CMD=CONNECT RSV=0", req[:3])
	}
	rest := req[4:]
	switch req[3] {
	case 1:
		if len(rest) != 4+2 {
			return "", 0, fmt.Errorf("IPv4 request has %d address and port bytes", len(rest))
		}
		host = netip.AddrFrom4([4]byte(rest[:4])).String()
		rest = rest[4:]
	case 4:
		if len(rest) != 16+2 {
			return "", 0, fmt.Errorf("IPv6 request has %d address and port bytes", len(rest))
		}
		host = netip.AddrFrom16([16]byte(rest[:16])).String()
		rest = rest[16:]
	case 3:
		if len(rest) < 1 || len(rest) != 1+int(rest[0])+2 {
			return "", 0, errors.New("domain length octet does not match the request")
		}
		host = string(rest[1 : 1+int(rest[0])])
		rest = rest[1+int(rest[0]):]
	default:
		return "", 0, fmt.Errorf("unknown ATYP %d", req[3])
	}
	return host, binary.BigEndian.Uint16(rest), nil
}

// domainAlphabet is what the domain-name generator draws from. Every name
// contains a letter, so net.ParseIP never reads it as an address.
const domainAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789-."

// domainFrom turns arbitrary bytes into a domain name of 1 to 255 bytes.
func domainFrom(raw []byte, length uint8) string {
	n := int(length)
	if n == 0 {
		n = 1
	}
	var b strings.Builder
	b.WriteByte('a' + length%26)
	for i := 1; i < n; i++ {
		var c byte
		if len(raw) > 0 {
			c = raw[i%len(raw)] + byte(i)
		} else {
			c = byte(i)
		}
		b.WriteByte(domainAlphabet[int(c)%len(domainAlphabet)])
	}
	return b.String()
}

func TestProperty_ConnectRequestDecodesToItsDomainAndPort(t *testing.T) {
	t.Parallel()

	checkProperty(t, func(raw []byte, length uint8, port uint16) bool {
		host := domainFrom(raw, length)
		req, err := buildConnectRequest(host, port)
		if err != nil {
			t.Logf("buildConnectRequest(%q, %d): %v", host, port, err)
			return false
		}
		gotHost, gotPort, err := decodeConnectRequest(req)
		return err == nil && req[3] == 3 && gotHost == host && gotPort == port
	})
}

func TestDifferential_ConnectRequestEncodesAddressesAsNetipReadsThem(t *testing.T) {
	t.Parallel()

	// netip is the reference for which bytes an address string stands for. An
	// IPv4-mapped IPv6 address goes out as the IPv4 address it maps.
	checkProperty(t, func(a [16]byte, v4 bool, port uint16) bool {
		addr := netip.AddrFrom16(a)
		if v4 {
			addr = netip.AddrFrom4([4]byte(a[:4]))
		}
		req, err := buildConnectRequest(addr.String(), port)
		if err != nil {
			t.Logf("buildConnectRequest(%q, %d): %v", addr, port, err)
			return false
		}
		gotHost, gotPort, err := decodeConnectRequest(req)
		if err != nil {
			t.Logf("decode %x: %v", req, err)
			return false
		}
		wantATYP := byte(4)
		if addr.Unmap().Is4() {
			wantATYP = 1
		}
		got, err := netip.ParseAddr(gotHost)
		return err == nil && req[3] == wantATYP && got == addr.Unmap() && gotPort == port
	})
}

func TestBoundary_ConnectRequestDomainLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		length int
		ok     bool
	}{
		{0, false},
		{1, true},
		{254, true},
		{255, true},
		{256, false},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.length), func(t *testing.T) {
			t.Parallel()
			host := strings.Repeat("a", tt.length)
			req, err := buildConnectRequest(host, 443)
			if (err == nil) != tt.ok {
				t.Fatalf("buildConnectRequest(%d bytes) error = %v, want ok=%v", tt.length, err, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got, _, err := decodeConnectRequest(req); err != nil || got != host {
				t.Fatalf("decoded %d-byte domain = %q, %v", tt.length, got, err)
			}
		})
	}
}

func TestBoundary_ParsePort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want uint16
		ok   bool
	}{
		{"-1", 0, false},
		{"0", 0, true},
		{"1", 1, true},
		{"65534", 65534, true},
		{"65535", 65535, true},
		{"65536", 0, false},
		{"", 0, false},
		{"http", 0, false},
		{"80a", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := parsePort(tt.in)
			if (err == nil) != tt.ok || got != tt.want {
				t.Fatalf("parsePort(%q) = %d, %v; want %d, ok=%v", tt.in, got, err, tt.want, tt.ok)
			}
		})
	}
}

func TestProperty_SuccessRateIsAlwaysInTheUnitInterval(t *testing.T) {
	t.Parallel()

	inUnit := func(r float64) bool {
		v := NewSuccessRate(r).Float64()
		return v >= 0 && v <= 1
	}
	for _, r := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), math.SmallestNonzeroFloat64, 1 + 1e-16, math.MaxFloat64} {
		if !inUnit(r) {
			t.Errorf("NewSuccessRate(%v) = %v, outside [0, 1]", r, NewSuccessRate(r).Float64())
		}
	}
	checkProperty(t, inUnit)
	checkProperty(t, func(r float64) bool {
		// A rate already inside the interval is kept as it is.
		r = math.Abs(math.Mod(r, 1))
		return NewSuccessRate(r).Float64() == r
	})
}

// measurement is one generated RelayStats input.
type measurement struct {
	latency time.Duration
	success bool
}

// measurementsFrom turns generated values into measurements. bound limits each
// latency to [0, bound); 0 means the full non-negative Duration range.
func measurementsFrom(lat []int64, ok []bool, bound int64) []measurement {
	n := min(len(lat), 64)
	ms := make([]measurement, n)
	for i := range n {
		d := lat[i]
		if d < 0 {
			d = -(d + 1)
		}
		if bound > 0 {
			d %= bound
		}
		ms[i] = measurement{latency: time.Duration(d), success: i < len(ok) && ok[i]}
	}
	return ms
}

func relayStatsOf(ms []measurement) *RelayStats {
	s := NewRelayStats(NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), time.Hour)
	for _, m := range ms {
		s.AddMeasurement(NewRelayMeasurement(m.latency, m.success))
	}
	return s
}

func TestDifferential_RelayStatsAverageIsTheFloorOfTheExactMean(t *testing.T) {
	t.Parallel()

	// math/big computes the mean without any overflow and is the reference.
	checkProperty(t, func(lat []int64, ok []bool) bool {
		ms := measurementsFrom(lat, ok, 0)
		if len(ms) == 0 {
			return relayStatsOf(ms).AverageLatency().Duration() == 0
		}
		sum := new(big.Int)
		var successes int
		for _, m := range ms {
			sum.Add(sum, big.NewInt(int64(m.latency)))
			if m.success {
				successes++
			}
		}
		want := sum.Quo(sum, big.NewInt(int64(len(ms)))).Int64()
		s := relayStatsOf(ms)
		return int64(s.AverageLatency().Duration()) == want &&
			s.SuccessRate().Float64() == float64(successes)/float64(len(ms)) &&
			s.SampleCount() == len(ms)
	})
}

func TestMetamorphic_RelayStatsDoNotDependOnMeasurementOrder(t *testing.T) {
	t.Parallel()

	threshold := NewRelayThreshold().WithMaxLatency(time.Second).WithMinSamples(1)
	checkProperty(t, func(lat []int64, ok []bool, seed uint64) bool {
		ms := measurementsFrom(lat, ok, int64(2*time.Second))
		shuffled := slices.Clone(ms)
		r := rand.New(rand.NewPCG(seed, seed)) //nolint:gosec // G404: a test's shuffle, not a secret
		r.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		a, b := relayStatsOf(ms), relayStatsOf(shuffled)
		return a.AverageLatency() == b.AverageLatency() &&
			a.SuccessRate() == b.SuccessRate() &&
			a.IsSlow(threshold) == b.IsSlow(threshold)
	})
}

func TestMetamorphic_RelayStatsAreSlowUnderAStricterThreshold(t *testing.T) {
	t.Parallel()

	// Lowering the latency limit or raising the success-rate floor can only
	// make more relays slow, never fewer.
	checkProperty(t, func(lat []int64, ok []bool, maxMs, cutMs uint16, minRate, raise float64) bool {
		ms := measurementsFrom(lat, ok, int64(3*time.Second))
		s := relayStatsOf(ms)
		maxLatency := time.Duration(maxMs) * time.Millisecond
		rate := math.Abs(math.Mod(minRate, 1))
		loose := NewRelayThreshold().WithMaxLatency(maxLatency).WithMinSuccessRate(rate).WithMinSamples(1)
		strict := loose.
			WithMaxLatency(maxLatency - min(time.Duration(cutMs)*time.Millisecond, maxLatency)).
			WithMinSuccessRate(rate + math.Abs(math.Mod(raise, 1))*(1-rate))
		return !s.IsSlow(loose) || s.IsSlow(strict)
	})
}

func TestProperty_MetricsCollectorAgreesWithTheRequestsItRecorded(t *testing.T) {
	t.Parallel()

	checkProperty(t, func(lat []uint32, failed []bool) bool {
		// A small range makes a latency of 0, and repeated latencies, common:
		// a fast request on a coarse clock measures 0.
		for i := range lat {
			lat[i] %= 1024
		}
		m := NewMetricsCollector()
		var total time.Duration
		var errs uint64
		for i, l := range lat {
			d := time.Duration(l)
			total += d
			var err error
			if i < len(failed) && failed[i] {
				err = newError(ErrHTTPFailed, opClient, "request failed", nil)
				errs++
			}
			m.recordRequest(d, err)
		}
		n := uint64(len(lat))
		if m.RequestCount() != n || m.ErrorCount() != errs || m.SuccessCount() != n-errs ||
			m.TotalLatency() != total || m.ErrorsByKind()[ErrHTTPFailed] != errs {
			return false
		}
		if n == 0 {
			return m.MinLatency() == 0 && m.MaxLatency() == 0 && m.AverageLatency() == 0
		}
		return m.MinLatency() == time.Duration(slices.Min(lat)) &&
			m.MaxLatency() == time.Duration(slices.Max(lat)) &&
			m.AverageLatency() == total/time.Duration(len(lat))
	})
}

func TestMetamorphic_MetricsResetIsLikeANewCollector(t *testing.T) {
	t.Parallel()

	checkProperty(t, func(before, after []uint32) bool {
		reset := NewMetricsCollector()
		for _, l := range before {
			reset.recordRequest(time.Duration(l), nil)
		}
		reset.Reset()
		fresh := NewMetricsCollector()
		for _, l := range after {
			reset.recordRequest(time.Duration(l), nil)
			fresh.recordRequest(time.Duration(l), nil)
		}
		return reset.RequestCount() == fresh.RequestCount() &&
			reset.TotalLatency() == fresh.TotalLatency() &&
			reset.MinLatency() == fresh.MinLatency() &&
			reset.MaxLatency() == fresh.MaxLatency()
	})
}

// tokenFrom turns arbitrary bytes into a non-empty token without spaces or
// '=' and that does not start with a keyword parseCircuitLine recognizes.
func tokenFrom(raw []byte, fallback string) string {
	var b strings.Builder
	for _, c := range raw {
		if c > ' ' && c < 0x7f && c != '=' {
			b.WriteByte(c)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return "x" + b.String()
}

func TestMetamorphic_CircuitLineIgnoresUnknownKeywordsAndTheirOrder(t *testing.T) {
	t.Parallel()

	checkProperty(t, func(id, status, path, purpose, extra []byte, reverse bool) bool {
		hops := strings.Split(tokenFrom(path, "$A~a"), ",")
		keywords := make([]string, 0, 4)
		keywords = append(keywords,
			"BUILD_FLAGS=NEED_CAPACITY,NEED_UPTIME",
			"PURPOSE="+tokenFrom(purpose, "GENERAL"),
			"TIME_CREATED=2026-11-19T00:00:00.000000",
		)
		line := strings.Join(append([]string{tokenFrom(id, "1"), tokenFrom(status, "BUILT"), strings.Join(hops, ",")}, keywords...), " ")
		want := parseCircuitLine(line)

		if reverse {
			slices.Reverse(keywords)
		}
		unknown := "UNKNOWN_" + strings.ToUpper(tokenFrom(extra, "K")) + "=v"
		variant := strings.Join(append([]string{tokenFrom(id, "1"), tokenFrom(status, "BUILT"), strings.Join(hops, ",")}, append(keywords, unknown)...), "  ")
		got := parseCircuitLine(variant)
		return reflect.DeepEqual(got, want) && reflect.DeepEqual(got.Path, hops)
	})
}

func TestMetamorphic_StreamLineIgnoresUnknownKeywords(t *testing.T) {
	t.Parallel()

	checkProperty(t, func(id, circ, purpose, extra []byte) bool {
		base := []string{tokenFrom(id, "7"), "SUCCEEDED", tokenFrom(circ, "3"), "example.com:443"}
		want := parseStreamLine(strings.Join(append(base, "PURPOSE="+tokenFrom(purpose, "USER")), " "))
		got := parseStreamLine(strings.Join(append(base, "SOURCE_ADDR="+tokenFrom(extra, "x"), "PURPOSE="+tokenFrom(purpose, "USER")), "\t"))
		return got == want && want.ID == base[0] && want.CircuitID == base[2]
	})
}

func TestProperty_AddOnionCommandListsEveryPortOnceInAscendingOrder(t *testing.T) {
	t.Parallel()

	checkProperty(t, func(ports map[uint16]uint16) bool {
		mapping := make(map[int]int, len(ports))
		for v, tgt := range ports {
			mapping[int(v)] = int(tgt)
		}
		cfg := HiddenServiceConfig{keyType: "ED25519-V3", targetPort: mapping}
		cmd := buildAddOnionCommand(cfg)
		if cmd != buildAddOnionCommand(cfg) || strings.ContainsAny(cmd, "\r\n") {
			return false
		}
		fields := strings.Fields(cmd)
		if len(fields) != 2+len(mapping) || fields[0] != "ADD_ONION" || fields[1] != "NEW:ED25519-V3" {
			return false
		}
		virts := make([]int, 0, len(mapping))
		for _, f := range fields[2:] {
			var v, tgt int
			if _, err := fmt.Sscanf(f, "Port=%d,127.0.0.1:%d", &v, &tgt); err != nil || mapping[v] != tgt {
				return false
			}
			virts = append(virts, v)
		}
		return sort.IntsAreSorted(virts) && len(slices.Compact(slices.Clone(virts))) == len(virts)
	})
}
