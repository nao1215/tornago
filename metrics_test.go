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
