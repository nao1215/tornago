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
	minLatency   int64 // nanoseconds
	maxLatency   int64 // nanoseconds
	mu           sync.RWMutex

	// Error tracking by kind
	errorsByKind map[ErrorKind]uint64
	errorsMu     sync.RWMutex

	// Connection reuse metrics
	dialCount uint64 // Total number of dial operations
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		errorsByKind: make(map[ErrorKind]uint64),
	}
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

// MinLatency returns the minimum request latency observed.
// Returns 0 if no requests have been made.
func (m *MetricsCollector) MinLatency() time.Duration {
	return time.Duration(atomic.LoadInt64(&m.minLatency))
}

// MaxLatency returns the maximum request latency observed.
// Returns 0 if no requests have been made.
func (m *MetricsCollector) MaxLatency() time.Duration {
	return time.Duration(atomic.LoadInt64(&m.maxLatency))
}

// ErrorsByKind returns a copy of error counts grouped by error kind.
func (m *MetricsCollector) ErrorsByKind() map[ErrorKind]uint64 {
	m.errorsMu.RLock()
	defer m.errorsMu.RUnlock()

	result := make(map[ErrorKind]uint64, len(m.errorsByKind))
	for k, v := range m.errorsByKind {
		result[k] = v
	}
	return result
}

// DialCount returns the total number of dial operations performed.
// This includes both new connections and connection reuse attempts.
func (m *MetricsCollector) DialCount() uint64 {
	return atomic.LoadUint64(&m.dialCount)
}

// ConnectionReuseCount returns the number of times an existing connection was reused.
// This is calculated as the difference between total requests and total dials.
func (m *MetricsCollector) ConnectionReuseCount() uint64 {
	requests := atomic.LoadUint64(&m.requestCount)
	dials := atomic.LoadUint64(&m.dialCount)
	if dials >= requests {
		return 0
	}
	return requests - dials
}

// ConnectionReuseRate returns the percentage of requests that reused existing connections.
// Returns 0.0 if no requests have been made.
// A higher rate (closer to 1.0) indicates better connection pooling efficiency.
func (m *MetricsCollector) ConnectionReuseRate() float64 {
	requests := atomic.LoadUint64(&m.requestCount)
	if requests == 0 {
		return 0.0
	}
	dials := atomic.LoadUint64(&m.dialCount)
	if dials >= requests {
		return 0.0
	}
	reused := requests - dials
	return float64(reused) / float64(requests)
}

// Reset clears all metrics to zero.
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomic.StoreUint64(&m.requestCount, 0)
	atomic.StoreUint64(&m.successCount, 0)
	atomic.StoreUint64(&m.errorCount, 0)
	atomic.StoreInt64(&m.totalLatency, 0)
	atomic.StoreInt64(&m.minLatency, 0)
	atomic.StoreInt64(&m.maxLatency, 0)
	atomic.StoreUint64(&m.dialCount, 0)

	m.errorsMu.Lock()
	m.errorsByKind = make(map[ErrorKind]uint64)
	m.errorsMu.Unlock()
}

// recordDial increments the dial count when a new connection is established.
func (m *MetricsCollector) recordDial() {
	atomic.AddUint64(&m.dialCount, 1)
}

// recordRequest increments the request count and records latency.
func (m *MetricsCollector) recordRequest(latency time.Duration, err error) {
	atomic.AddUint64(&m.requestCount, 1)
	latencyNs := int64(latency)
	atomic.AddInt64(&m.totalLatency, latencyNs)

	// Update min/max latency
	for {
		oldMin := atomic.LoadInt64(&m.minLatency)
		if oldMin != 0 && latencyNs >= oldMin {
			break
		}
		if atomic.CompareAndSwapInt64(&m.minLatency, oldMin, latencyNs) {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&m.maxLatency)
		if latencyNs <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.maxLatency, oldMax, latencyNs) {
			break
		}
	}

	if err == nil {
		atomic.AddUint64(&m.successCount, 1)
	} else {
		atomic.AddUint64(&m.errorCount, 1)

		// Track error by kind
		var torErr *TornagoError
		if As(err, &torErr) {
			m.errorsMu.Lock()
			m.errorsByKind[torErr.Kind]++
			m.errorsMu.Unlock()
		}
	}
}

// As is a helper function that wraps errors.As for internal use.
func As(err error, target any) bool {
	if err == nil {
		return false
	}
	targetPtr, ok := target.(**TornagoError)
	if !ok {
		return false
	}
	for err != nil {
		if torErr, ok := err.(*TornagoError); ok { //nolint:errorlint // intentional type assertion
			*targetPtr = torErr
			return true
		}
		type unwrapper interface {
			Unwrap() error
		}
		if u, ok := err.(unwrapper); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
