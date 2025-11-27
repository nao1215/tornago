package tornago

import (
	"context"
	"testing"
	"time"
)

func TestNewRelayFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain fingerprint",
			input:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			expected: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			name:     "fingerprint with $ prefix",
			input:    "$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			expected: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			name:     "fingerprint with name suffix",
			input:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA~MyRelay",
			expected: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			name:     "fingerprint with $ prefix and name suffix",
			input:    "$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA~MyRelay",
			expected: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			name:     "empty fingerprint",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fp := NewRelayFingerprint(tt.input)
			if fp.String() != tt.expected {
				t.Errorf("NewRelayFingerprint(%q) = %q, want %q", tt.input, fp.String(), tt.expected)
			}
		})
	}
}

func TestRelayFingerprintEqual(t *testing.T) {
	t.Parallel()

	fp1 := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	fp2 := NewRelayFingerprint("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	fp3 := NewRelayFingerprint("BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")

	if !fp1.Equal(fp2) {
		t.Error("fingerprints should be equal (case insensitive)")
	}

	if fp1.Equal(fp3) {
		t.Error("different fingerprints should not be equal")
	}
}

func TestRelayFingerprintIsEmpty(t *testing.T) {
	t.Parallel()

	empty := NewRelayFingerprint("")
	nonEmpty := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	if !empty.IsEmpty() {
		t.Error("empty fingerprint should return true for IsEmpty")
	}

	if nonEmpty.IsEmpty() {
		t.Error("non-empty fingerprint should return false for IsEmpty")
	}
}

func TestNewLatency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{
			name:     "positive duration",
			input:    5 * time.Second,
			expected: 5 * time.Second,
		},
		{
			name:     "zero duration",
			input:    0,
			expected: 0,
		},
		{
			name:     "negative duration becomes zero",
			input:    -5 * time.Second,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := NewLatency(tt.input)
			if l.Duration() != tt.expected {
				t.Errorf("NewLatency(%v).Duration() = %v, want %v", tt.input, l.Duration(), tt.expected)
			}
		})
	}
}

func TestLatencyExceedsThreshold(t *testing.T) {
	t.Parallel()

	l1 := NewLatency(5 * time.Second)
	l2 := NewLatency(3 * time.Second)
	threshold := NewLatency(4 * time.Second)

	if !l1.ExceedsThreshold(threshold) {
		t.Error("5s should exceed 4s threshold")
	}

	if l2.ExceedsThreshold(threshold) {
		t.Error("3s should not exceed 4s threshold")
	}
}

func TestNewSuccessRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "normal rate",
			input:    0.85,
			expected: 0.85,
		},
		{
			name:     "rate clamped to zero",
			input:    -0.5,
			expected: 0,
		},
		{
			name:     "rate clamped to one",
			input:    1.5,
			expected: 1,
		},
		{
			name:     "zero rate",
			input:    0,
			expected: 0,
		},
		{
			name:     "one rate",
			input:    1,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := NewSuccessRate(tt.input)
			if s.Float64() != tt.expected {
				t.Errorf("NewSuccessRate(%v).Float64() = %v, want %v", tt.input, s.Float64(), tt.expected)
			}
		})
	}
}

func TestSuccessRateBelowThreshold(t *testing.T) {
	t.Parallel()

	s1 := NewSuccessRate(0.7)
	s2 := NewSuccessRate(0.9)
	threshold := NewSuccessRate(0.8)

	if !s1.BelowThreshold(threshold) {
		t.Error("0.7 should be below 0.8 threshold")
	}

	if s2.BelowThreshold(threshold) {
		t.Error("0.9 should not be below 0.8 threshold")
	}
}

func TestRelayThreshold(t *testing.T) {
	t.Parallel()

	// Test default values
	defaultThreshold := NewRelayThreshold()
	if defaultThreshold.MaxLatency().Duration() != defaultLatencyThreshold {
		t.Errorf("default maxLatency = %v, want %v", defaultThreshold.MaxLatency().Duration(), defaultLatencyThreshold)
	}
	if defaultThreshold.MinSuccessRate().Float64() != defaultMinSuccessRate {
		t.Errorf("default minSuccessRate = %v, want %v", defaultThreshold.MinSuccessRate().Float64(), defaultMinSuccessRate)
	}
	if defaultThreshold.BlockDuration() != defaultBlockDuration {
		t.Errorf("default blockDuration = %v, want %v", defaultThreshold.BlockDuration(), defaultBlockDuration)
	}
	if defaultThreshold.MinSamples() != defaultMinSamples {
		t.Errorf("default minSamples = %d, want %d", defaultThreshold.MinSamples(), defaultMinSamples)
	}
}

func TestRelayThresholdWithMethods(t *testing.T) {
	t.Parallel()

	threshold := NewRelayThreshold().
		WithMaxLatency(10 * time.Second).
		WithMinSuccessRate(0.9).
		WithBlockDuration(1 * time.Hour).
		WithMinSamples(5)

	if threshold.MaxLatency().Duration() != 10*time.Second {
		t.Errorf("maxLatency = %v, want %v", threshold.MaxLatency().Duration(), 10*time.Second)
	}
	if threshold.MinSuccessRate().Float64() != 0.9 {
		t.Errorf("minSuccessRate = %v, want %v", threshold.MinSuccessRate().Float64(), 0.9)
	}
	if threshold.BlockDuration() != 1*time.Hour {
		t.Errorf("blockDuration = %v, want %v", threshold.BlockDuration(), 1*time.Hour)
	}
	if threshold.MinSamples() != 5 {
		t.Errorf("minSamples = %d, want %d", threshold.MinSamples(), 5)
	}
}

func TestRelayThresholdWithMinSamplesClamp(t *testing.T) {
	t.Parallel()

	threshold := NewRelayThreshold().WithMinSamples(0)
	if threshold.MinSamples() != 1 {
		t.Errorf("minSamples should be clamped to 1, got %d", threshold.MinSamples())
	}

	threshold = NewRelayThreshold().WithMinSamples(-5)
	if threshold.MinSamples() != 1 {
		t.Errorf("negative minSamples should be clamped to 1, got %d", threshold.MinSamples())
	}
}

func TestRelayMeasurement(t *testing.T) {
	t.Parallel()

	m := NewRelayMeasurement(100*time.Millisecond, true)

	if m.Latency().Duration() != 100*time.Millisecond {
		t.Errorf("latency = %v, want %v", m.Latency().Duration(), 100*time.Millisecond)
	}
	if !m.Success() {
		t.Error("success should be true")
	}
	if m.Timestamp().IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestRelayMeasurementIsExpired(t *testing.T) {
	t.Parallel()

	m := NewRelayMeasurement(100*time.Millisecond, true)

	// Not expired immediately
	if m.IsExpired(10 * time.Minute) {
		t.Error("measurement should not be expired immediately")
	}

	// Wait a small amount of time to ensure time has passed
	// This is necessary because on some platforms (e.g., Windows),
	// timer precision may cause time.Since to return 0 immediately after creation.
	time.Sleep(1 * time.Millisecond)

	// Expired with very short window (after waiting)
	if !m.IsExpired(0) {
		t.Error("measurement should be expired with zero window after waiting")
	}
}

func TestRelayStats(t *testing.T) {
	t.Parallel()

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	stats := NewRelayStats(fp, 10*time.Minute)

	if !stats.Fingerprint().Equal(fp) {
		t.Error("fingerprint mismatch")
	}
	if stats.SampleCount() != 0 {
		t.Errorf("initial sample count should be 0, got %d", stats.SampleCount())
	}
}

func TestRelayStatsAddMeasurement(t *testing.T) {
	t.Parallel()

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	stats := NewRelayStats(fp, 10*time.Minute)

	// Add successful measurements
	stats.AddMeasurement(NewRelayMeasurement(100*time.Millisecond, true))
	stats.AddMeasurement(NewRelayMeasurement(200*time.Millisecond, true))
	stats.AddMeasurement(NewRelayMeasurement(300*time.Millisecond, false))

	if stats.SampleCount() != 3 {
		t.Errorf("sample count = %d, want 3", stats.SampleCount())
	}

	// Average latency: (100 + 200 + 300) / 3 = 200ms
	if stats.AverageLatency().Duration() != 200*time.Millisecond {
		t.Errorf("average latency = %v, want %v", stats.AverageLatency().Duration(), 200*time.Millisecond)
	}

	// Success rate: 2/3 ≈ 0.666...
	expectedRate := 2.0 / 3.0
	if stats.SuccessRate().Float64() != expectedRate {
		t.Errorf("success rate = %v, want %v", stats.SuccessRate().Float64(), expectedRate)
	}
}

func TestRelayStatsIsSlow(t *testing.T) {
	t.Parallel()

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	stats := NewRelayStats(fp, 10*time.Minute)

	threshold := NewRelayThreshold().
		WithMaxLatency(500 * time.Millisecond).
		WithMinSuccessRate(0.8).
		WithMinSamples(3)

	// Not enough samples
	stats.AddMeasurement(NewRelayMeasurement(1*time.Second, true))
	if stats.IsSlow(threshold) {
		t.Error("should not be slow with insufficient samples")
	}

	// Add more samples (all fast and successful)
	stats.AddMeasurement(NewRelayMeasurement(100*time.Millisecond, true))
	stats.AddMeasurement(NewRelayMeasurement(100*time.Millisecond, true))

	if stats.IsSlow(threshold) {
		t.Error("should not be slow with good performance")
	}
}

func TestRelayStatsIsSlowHighLatency(t *testing.T) {
	t.Parallel()

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	stats := NewRelayStats(fp, 10*time.Minute)

	threshold := NewRelayThreshold().
		WithMaxLatency(500 * time.Millisecond).
		WithMinSuccessRate(0.5).
		WithMinSamples(3)

	// Add slow measurements
	stats.AddMeasurement(NewRelayMeasurement(1*time.Second, true))
	stats.AddMeasurement(NewRelayMeasurement(1*time.Second, true))
	stats.AddMeasurement(NewRelayMeasurement(1*time.Second, true))

	if !stats.IsSlow(threshold) {
		t.Error("should be slow due to high latency")
	}
}

func TestRelayStatsIsSlowLowSuccessRate(t *testing.T) {
	t.Parallel()

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	stats := NewRelayStats(fp, 10*time.Minute)

	threshold := NewRelayThreshold().
		WithMaxLatency(5 * time.Second).
		WithMinSuccessRate(0.8).
		WithMinSamples(3)

	// Add measurements with low success rate
	stats.AddMeasurement(NewRelayMeasurement(100*time.Millisecond, true))
	stats.AddMeasurement(NewRelayMeasurement(100*time.Millisecond, false))
	stats.AddMeasurement(NewRelayMeasurement(100*time.Millisecond, false))

	if !stats.IsSlow(threshold) {
		t.Error("should be slow due to low success rate")
	}
}

func TestRelayPerformanceTracker(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker()

	if tracker == nil {
		t.Fatal("tracker should not be nil")
	}

	stats := tracker.Stats()
	if stats.TrackedRelays != 0 {
		t.Errorf("initial tracked relays = %d, want 0", stats.TrackedRelays)
	}
	if stats.BlockedRelays != 0 {
		t.Errorf("initial blocked relays = %d, want 0", stats.BlockedRelays)
	}
}

func TestRelayPerformanceTrackerWithOptions(t *testing.T) {
	t.Parallel()

	threshold := NewRelayThreshold().WithMaxLatency(1 * time.Second)
	tracker := NewRelayPerformanceTracker(
		WithTrackerThreshold(threshold),
		WithTrackerMeasureWindow(5*time.Minute),
		WithTrackerAutoExclude(false),
	)

	stats := tracker.Stats()
	if stats.Threshold.MaxLatency().Duration() != 1*time.Second {
		t.Errorf("threshold maxLatency = %v, want %v", stats.Threshold.MaxLatency().Duration(), 1*time.Second)
	}
}

func TestRelayPerformanceTrackerRecordMeasurement(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker()
	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	tracker.RecordMeasurement(fp, 100*time.Millisecond, true)

	stats := tracker.GetStats(fp)
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.SampleCount() != 1 {
		t.Errorf("sample count = %d, want 1", stats.SampleCount())
	}
}

func TestRelayPerformanceTrackerRecordCircuitMeasurement(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker()
	path := []string{
		"$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"$BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		"$CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
	}

	tracker.RecordCircuitMeasurement(path, 500*time.Millisecond, true)

	for _, fp := range path {
		fingerprint := NewRelayFingerprint(fp)
		stats := tracker.GetStats(fingerprint)
		if stats == nil {
			t.Errorf("stats for %s should not be nil", fp)
			continue
		}
		if stats.SampleCount() != 1 {
			t.Errorf("sample count for %s = %d, want 1", fp, stats.SampleCount())
		}
	}
}

func TestRelayPerformanceTrackerEmptyFingerprint(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker()
	fp := NewRelayFingerprint("")

	// Should not panic and should not record anything
	tracker.RecordMeasurement(fp, 100*time.Millisecond, true)

	stats := tracker.Stats()
	if stats.TrackedRelays != 0 {
		t.Errorf("tracked relays = %d, want 0 (empty fingerprint should be ignored)", stats.TrackedRelays)
	}
}

func TestRelayPerformanceTrackerBlockRelay(t *testing.T) {
	t.Parallel()

	threshold := NewRelayThreshold().
		WithMaxLatency(100 * time.Millisecond).
		WithMinSuccessRate(0.8).
		WithMinSamples(3).
		WithBlockDuration(1 * time.Hour)

	tracker := NewRelayPerformanceTracker(
		WithTrackerThreshold(threshold),
		WithTrackerAutoExclude(false), // Disable auto exclude for testing
	)

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	// Add slow measurements to trigger blocking
	tracker.RecordMeasurement(fp, 1*time.Second, true)
	tracker.RecordMeasurement(fp, 1*time.Second, true)
	tracker.RecordMeasurement(fp, 1*time.Second, true)

	if !tracker.IsBlocked(fp) {
		t.Error("relay should be blocked due to high latency")
	}

	blocked := tracker.BlockedRelays()
	if len(blocked) != 1 {
		t.Errorf("blocked relays count = %d, want 1", len(blocked))
	}
}

func TestRelayPerformanceTrackerClear(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker(
		WithTrackerAutoExclude(false),
	)
	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	tracker.RecordMeasurement(fp, 100*time.Millisecond, true)
	tracker.Clear()

	stats := tracker.Stats()
	if stats.TrackedRelays != 0 {
		t.Errorf("tracked relays after clear = %d, want 0", stats.TrackedRelays)
	}
}

func TestRelayPerformanceTrackerIsBlockedNonExistent(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker()
	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	if tracker.IsBlocked(fp) {
		t.Error("non-existent relay should not be blocked")
	}
}

func TestLatencyIsZero(t *testing.T) {
	t.Parallel()

	zero := NewLatency(0)
	nonZero := NewLatency(100 * time.Millisecond)

	if !zero.IsZero() {
		t.Error("zero latency should return true for IsZero")
	}

	if nonZero.IsZero() {
		t.Error("non-zero latency should return false for IsZero")
	}
}

func TestRelayStatsWithZeroMeasureWindow(t *testing.T) {
	t.Parallel()

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	stats := NewRelayStats(fp, 0) // zero window should use default

	if stats == nil {
		t.Fatal("stats should not be nil")
	}
}

func TestTrackerStatsThreshold(t *testing.T) {
	t.Parallel()

	threshold := NewRelayThreshold().
		WithMaxLatency(2 * time.Second).
		WithMinSuccessRate(0.7)

	tracker := NewRelayPerformanceTracker(
		WithTrackerThreshold(threshold),
	)

	stats := tracker.Stats()
	if stats.Threshold.MaxLatency().Duration() != 2*time.Second {
		t.Errorf("threshold maxLatency = %v, want %v", stats.Threshold.MaxLatency().Duration(), 2*time.Second)
	}
	if stats.Threshold.MinSuccessRate().Float64() != 0.7 {
		t.Errorf("threshold minSuccessRate = %v, want %v", stats.Threshold.MinSuccessRate().Float64(), 0.7)
	}
}

func TestSlowRelayOptions(t *testing.T) {
	t.Parallel()

	t.Run("SlowRelayBlockDuration", func(t *testing.T) {
		t.Parallel()
		cfg := newSlowRelayConfig()
		SlowRelayBlockDuration(1 * time.Hour)(cfg)
		if cfg.blockDuration != 1*time.Hour {
			t.Errorf("blockDuration = %v, want %v", cfg.blockDuration, 1*time.Hour)
		}
	})

	t.Run("SlowRelayMinSamples", func(t *testing.T) {
		t.Parallel()
		cfg := newSlowRelayConfig()
		SlowRelayMinSamples(10)(cfg)
		if cfg.minSamples != 10 {
			t.Errorf("minSamples = %d, want %d", cfg.minSamples, 10)
		}
	})

	t.Run("SlowRelayMonitorInterval", func(t *testing.T) {
		t.Parallel()
		cfg := newSlowRelayConfig()
		SlowRelayMonitorInterval(15 * time.Second)(cfg)
		if cfg.monitorInterval != 15*time.Second {
			t.Errorf("monitorInterval = %v, want %v", cfg.monitorInterval, 15*time.Second)
		}
	})

	t.Run("SlowRelayAutoExclude", func(t *testing.T) {
		t.Parallel()
		cfg := newSlowRelayConfig()
		SlowRelayAutoExclude(false)(cfg)
		if cfg.autoExclude != false {
			t.Errorf("autoExclude = %v, want %v", cfg.autoExclude, false)
		}
	})
}

func TestClearExcludeNodesWithoutControl(t *testing.T) {
	t.Parallel()

	// Tracker without control should return nil (no-op)
	tracker := NewRelayPerformanceTracker()
	ctx := context.Background()
	err := tracker.ClearExcludeNodes(ctx)
	if err != nil {
		t.Errorf("ClearExcludeNodes without control = %v, want nil", err)
	}
}

func TestGetBlockedFingerprintsWithExpired(t *testing.T) {
	t.Parallel()

	tracker := NewRelayPerformanceTracker(
		WithTrackerThreshold(NewRelayThreshold().
			WithMaxLatency(100 * time.Millisecond).
			WithMinSamples(1).
			WithBlockDuration(1 * time.Millisecond)), // Very short block duration
	)

	fp := NewRelayFingerprint("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	// Record slow measurements to trigger blocking
	tracker.RecordMeasurement(fp, 200*time.Millisecond, true)

	// Should be blocked initially
	if !tracker.IsBlocked(fp) {
		t.Error("relay should be blocked after slow measurement")
	}

	// Wait for block to expire
	time.Sleep(5 * time.Millisecond)

	// Should no longer be blocked
	if tracker.IsBlocked(fp) {
		t.Error("relay should not be blocked after expiration")
	}
}

func TestRelayPerformanceStatsAccessors(t *testing.T) {
	t.Parallel()

	stats := RelayPerformanceStats{
		enabled:          true,
		trackedRelays:    5,
		blockedRelays:    2,
		blockedRelayList: []string{"fp1", "fp2"},
		threshold: RelayThresholdStats{
			maxLatency:     3 * time.Second,
			minSuccessRate: 0.85,
			blockDuration:  20 * time.Minute,
			minSamples:     4,
		},
	}

	t.Run("Enabled", func(t *testing.T) {
		t.Parallel()
		if !stats.Enabled() {
			t.Error("Enabled() = false, want true")
		}
	})

	t.Run("TrackedRelays", func(t *testing.T) {
		t.Parallel()
		if stats.TrackedRelays() != 5 {
			t.Errorf("TrackedRelays() = %d, want %d", stats.TrackedRelays(), 5)
		}
	})

	t.Run("BlockedRelays", func(t *testing.T) {
		t.Parallel()
		if stats.BlockedRelays() != 2 {
			t.Errorf("BlockedRelays() = %d, want %d", stats.BlockedRelays(), 2)
		}
	})

	t.Run("BlockedRelayList returns copy", func(t *testing.T) {
		t.Parallel()
		list := stats.BlockedRelayList()
		if len(list) != 2 {
			t.Errorf("len(BlockedRelayList()) = %d, want %d", len(list), 2)
		}
		// Modify returned list should not affect original
		list[0] = "modified"
		originalList := stats.BlockedRelayList()
		if originalList[0] == "modified" {
			t.Error("BlockedRelayList should return a copy, not the original slice")
		}
	})

	t.Run("BlockedRelayList nil case", func(t *testing.T) {
		t.Parallel()
		emptyStats := RelayPerformanceStats{}
		list := emptyStats.BlockedRelayList()
		if list != nil {
			t.Errorf("BlockedRelayList() on empty stats = %v, want nil", list)
		}
	})

	t.Run("Threshold", func(t *testing.T) {
		t.Parallel()
		threshold := stats.Threshold()
		if threshold.MaxLatency() != 3*time.Second {
			t.Errorf("Threshold().MaxLatency() = %v, want %v", threshold.MaxLatency(), 3*time.Second)
		}
		if threshold.MinSuccessRate() != 0.85 {
			t.Errorf("Threshold().MinSuccessRate() = %v, want %v", threshold.MinSuccessRate(), 0.85)
		}
		if threshold.BlockDuration() != 20*time.Minute {
			t.Errorf("Threshold().BlockDuration() = %v, want %v", threshold.BlockDuration(), 20*time.Minute)
		}
		if threshold.MinSamples() != 4 {
			t.Errorf("Threshold().MinSamples() = %d, want %d", threshold.MinSamples(), 4)
		}
	})
}
