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
}
