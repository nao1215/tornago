package tornago

import (
	"errors"
	"testing"
	"time"
)

func TestMetricsCollector_Initial(t *testing.T) {
	m := NewMetricsCollector()
	if m.RequestCount() != 0 {
		t.Errorf("expected 0, got %d", m.RequestCount())
	}
	if m.SuccessCount() != 0 {
		t.Errorf("expected 0, got %d", m.SuccessCount())
	}
	if m.ErrorCount() != 0 {
		t.Errorf("expected 0, got %d", m.ErrorCount())
	}
	if m.TotalLatency() != 0 {
		t.Errorf("expected 0, got %v", m.TotalLatency())
	}
	if m.AverageLatency() != 0 {
		t.Errorf("expected 0, got %v", m.AverageLatency())
	}
}

func TestMetricsCollector_RecordSuccess(t *testing.T) {
	m := NewMetricsCollector()
	m.recordRequest(100*time.Millisecond, nil)

	if m.RequestCount() != 1 {
		t.Errorf("expected 1, got %d", m.RequestCount())
	}
	if m.SuccessCount() != 1 {
		t.Errorf("expected 1, got %d", m.SuccessCount())
	}
	if m.ErrorCount() != 0 {
		t.Errorf("expected 0, got %d", m.ErrorCount())
	}
	if m.TotalLatency() != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", m.TotalLatency())
	}
	if m.AverageLatency() != 100*time.Millisecond {
		t.Errorf("expected 100ms, got %v", m.AverageLatency())
	}
}

func TestMetricsCollector_RecordError(t *testing.T) {
	m := NewMetricsCollector()
	m.recordRequest(50*time.Millisecond, errors.New("test error"))

	if m.RequestCount() != 1 {
		t.Errorf("expected 1, got %d", m.RequestCount())
	}
	if m.SuccessCount() != 0 {
		t.Errorf("expected 0, got %d", m.SuccessCount())
	}
	if m.ErrorCount() != 1 {
		t.Errorf("expected 1, got %d", m.ErrorCount())
	}
}

func TestMetricsCollector_ErrorsByKindFindsAKindInsideAJoinedError(t *testing.T) {
	t.Parallel()

	m := NewMetricsCollector()
	joined := errors.Join(errors.New("close failed"), newError(ErrTimeout, "op", "timed out", nil))
	m.recordRequest(time.Millisecond, joined)

	if got := m.ErrorsByKind()[ErrTimeout]; got != 1 {
		t.Errorf("ErrorsByKind()[ErrTimeout] = %d, want 1", got)
	}
}

func TestMetricsCollector_MultipleRecords(t *testing.T) {
	m := NewMetricsCollector()
	m.recordRequest(100*time.Millisecond, nil)
	m.recordRequest(200*time.Millisecond, nil)
	m.recordRequest(300*time.Millisecond, errors.New("err"))

	if m.RequestCount() != 3 {
		t.Errorf("expected 3, got %d", m.RequestCount())
	}
	if m.SuccessCount() != 2 {
		t.Errorf("expected 2, got %d", m.SuccessCount())
	}
	if m.ErrorCount() != 1 {
		t.Errorf("expected 1, got %d", m.ErrorCount())
	}
	if m.TotalLatency() != 600*time.Millisecond {
		t.Errorf("expected 600ms, got %v", m.TotalLatency())
	}
	if m.AverageLatency() != 200*time.Millisecond {
		t.Errorf("expected 200ms, got %v", m.AverageLatency())
	}
}

func TestMetricsCollector_MinLatencyKeepsAZeroLatencyRequest(t *testing.T) {
	t.Parallel()

	// A clock with coarse resolution measures a fast request as 0. That 0 is
	// the smallest latency seen and must stay the minimum after slower ones.
	m := NewMetricsCollector()
	m.recordRequest(0, nil)
	m.recordRequest(5*time.Millisecond, nil)
	m.recordRequest(3*time.Millisecond, nil)

	if got := m.MinLatency(); got != 0 {
		t.Errorf("MinLatency() = %v, want 0", got)
	}
	if got := m.MaxLatency(); got != 5*time.Millisecond {
		t.Errorf("MaxLatency() = %v, want 5ms", got)
	}

	m.Reset()
	m.recordRequest(7*time.Millisecond, nil)
	if got := m.MinLatency(); got != 7*time.Millisecond {
		t.Errorf("MinLatency() after Reset = %v, want 7ms", got)
	}
}

func TestMetricsCollector_Reset(t *testing.T) {
	m := NewMetricsCollector()
	m.recordRequest(100*time.Millisecond, nil)
	m.recordRequest(200*time.Millisecond, errors.New("err"))
	m.recordDial()

	m.Reset()

	if m.RequestCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", m.RequestCount())
	}
	if m.SuccessCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", m.SuccessCount())
	}
	if m.ErrorCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", m.ErrorCount())
	}
	if m.TotalLatency() != 0 {
		t.Errorf("expected 0 after reset, got %v", m.TotalLatency())
	}
	if m.DialCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", m.DialCount())
	}
}

func TestMetricsCollector_ConnectionReuse(t *testing.T) {
	t.Parallel()

	m := NewMetricsCollector()

	// Initially no connections
	if m.DialCount() != 0 {
		t.Errorf("initial DialCount() = %d, want 0", m.DialCount())
	}
	if m.ConnectionReuseCount() != 0 {
		t.Errorf("initial ConnectionReuseCount() = %d, want 0", m.ConnectionReuseCount())
	}
	if m.ConnectionReuseRate() != 0.0 {
		t.Errorf("initial ConnectionReuseRate() = %f, want 0.0", m.ConnectionReuseRate())
	}

	// Scenario 1: 5 requests, 2 dials (3 reused)
	m.recordDial() // First connection
	m.recordRequest(100*time.Millisecond, nil)
	m.recordRequest(100*time.Millisecond, nil) // Reused
	m.recordDial()                             // Second connection
	m.recordRequest(100*time.Millisecond, nil)
	m.recordRequest(100*time.Millisecond, nil) // Reused
	m.recordRequest(100*time.Millisecond, nil) // Reused

	if m.DialCount() != 2 {
		t.Errorf("DialCount() = %d, want 2", m.DialCount())
	}
	if m.RequestCount() != 5 {
		t.Errorf("RequestCount() = %d, want 5", m.RequestCount())
	}
	// 5 requests - 2 dials = 3 reused
	expectedReuse := uint64(3)
	if m.ConnectionReuseCount() != expectedReuse {
		t.Errorf("ConnectionReuseCount() = %d, want %d", m.ConnectionReuseCount(), expectedReuse)
	}

	// Reuse rate = 3/5 = 0.6
	expectedRate := 0.6
	if rate := m.ConnectionReuseRate(); rate != expectedRate {
		t.Errorf("ConnectionReuseRate() = %f, want %f", rate, expectedRate)
	}
}

func TestMetricsCollector_ConnectionReuseRate_NoRequests(t *testing.T) {
	t.Parallel()

	m := NewMetricsCollector()

	// With no requests, rate should be 0
	if m.ConnectionReuseRate() != 0.0 {
		t.Errorf("ConnectionReuseRate() with no requests = %f, want 0.0", m.ConnectionReuseRate())
	}
}

func TestMetricsCollector_ConnectionReuseRate_MoreDialsThanRequests(t *testing.T) {
	t.Parallel()

	m := NewMetricsCollector()

	// Edge case: more dials than requests (shouldn't happen in reality)
	m.recordDial()
	m.recordDial()
	m.recordRequest(100*time.Millisecond, nil)

	// Rate should be 0 when dials >= requests
	if m.ConnectionReuseRate() != 0.0 {
		t.Errorf("ConnectionReuseRate() with dials >= requests = %f, want 0.0", m.ConnectionReuseRate())
	}
}

func TestMetricsCollector_DialCount(t *testing.T) {
	t.Parallel()

	m := NewMetricsCollector()

	m.recordDial()
	if m.DialCount() != 1 {
		t.Errorf("DialCount() = %d, want 1", m.DialCount())
	}

	m.recordDial()
	m.recordDial()
	if m.DialCount() != 3 {
		t.Errorf("DialCount() = %d, want 3", m.DialCount())
	}
}
