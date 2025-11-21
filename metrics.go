package tornago

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics provides access to client operation statistics.
// All methods are safe for concurrent use.
type Metrics interface {
	// RequestCount returns the total number of requests made.
	RequestCount() uint64
	// SuccessCount returns the number of successful requests.
	SuccessCount() uint64
	// ErrorCount returns the number of failed requests.
	ErrorCount() uint64
	// TotalLatency returns the sum of all request latencies.
	TotalLatency() time.Duration
	// AverageLatency returns the average request latency.
	AverageLatency() time.Duration
	// Reset clears all metrics.
	Reset()
}

// MetricsCollector tracks request statistics for the Client.
// It is thread-safe and can be shared across goroutines.
type MetricsCollector struct {
	requestCount uint64
	successCount uint64
	errorCount   uint64
	totalLatency int64 // nanoseconds
	mu           sync.RWMutex
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

// RequestCount returns the total number of requests made.
func (m *MetricsCollector) RequestCount() uint64 {
	return atomic.LoadUint64(&m.requestCount)
}

// SuccessCount returns the number of successful requests.
func (m *MetricsCollector) SuccessCount() uint64 {
	return atomic.LoadUint64(&m.successCount)
}

// ErrorCount returns the number of failed requests.
func (m *MetricsCollector) ErrorCount() uint64 {
	return atomic.LoadUint64(&m.errorCount)
}

// TotalLatency returns the sum of all request latencies.
func (m *MetricsCollector) TotalLatency() time.Duration {
	return time.Duration(atomic.LoadInt64(&m.totalLatency))
}

// AverageLatency returns the average request latency.
// Returns 0 if no requests have been made.
func (m *MetricsCollector) AverageLatency() time.Duration {
	count := atomic.LoadUint64(&m.requestCount)
	if count == 0 {
		return 0
	}
	total := atomic.LoadInt64(&m.totalLatency)
	return time.Duration(total) / time.Duration(count) //nolint:gosec // count is guaranteed > 0 and overflow is acceptable for metrics
}

// Reset clears all metrics to zero.
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomic.StoreUint64(&m.requestCount, 0)
	atomic.StoreUint64(&m.successCount, 0)
	atomic.StoreUint64(&m.errorCount, 0)
	atomic.StoreInt64(&m.totalLatency, 0)
}

// recordRequest increments the request count and records latency.
func (m *MetricsCollector) recordRequest(latency time.Duration, err error) {
	atomic.AddUint64(&m.requestCount, 1)
	atomic.AddInt64(&m.totalLatency, int64(latency))
	if err == nil {
		atomic.AddUint64(&m.successCount, 1)
	} else {
		atomic.AddUint64(&m.errorCount, 1)
	}
}
