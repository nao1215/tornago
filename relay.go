package tornago

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	// defaultLatencyThreshold is the default maximum acceptable latency.
	defaultLatencyThreshold = 5 * time.Second
	// defaultMinSuccessRate is the default minimum acceptable success rate.
	defaultMinSuccessRate = 0.8
	// defaultBlockDuration is how long to block slow relays.
	defaultBlockDuration = 30 * time.Minute
	// defaultMinSamples is the minimum number of samples needed for evaluation.
	defaultMinSamples = 3
	// defaultMeasurementWindow is the time window for measurements.
	defaultMeasurementWindow = 10 * time.Minute
)

// RelayFingerprint represents a Tor relay's unique identifier.
// It is a 40-character hexadecimal string (SHA-1 hash of the relay's identity key).
type RelayFingerprint struct {
	value string
}

// NewRelayFingerprint creates a new RelayFingerprint from a string.
// The fingerprint should be a 40-character hexadecimal string, optionally prefixed with '$'.
func NewRelayFingerprint(fingerprint string) RelayFingerprint {
	// Remove '$' prefix if present (Tor sometimes includes it)
	fp := strings.TrimPrefix(fingerprint, "$")
	// Remove any trailing flags like ~Name
	if idx := strings.Index(fp, "~"); idx != -1 {
		fp = fp[:idx]
	}
	return RelayFingerprint{value: fp}
}

// String returns the fingerprint as a string.
func (f RelayFingerprint) String() string {
	return f.value
}

// IsEmpty returns true if the fingerprint is empty.
func (f RelayFingerprint) IsEmpty() bool {
	return f.value == ""
}

// Equal returns true if two fingerprints are equal.
func (f RelayFingerprint) Equal(other RelayFingerprint) bool {
	return strings.EqualFold(f.value, other.value)
}

// Latency represents a duration measurement for relay performance.
type Latency struct {
	value time.Duration
}

// NewLatency creates a new Latency from a duration.
func NewLatency(d time.Duration) Latency {
	if d < 0 {
		d = 0
	}
	return Latency{value: d}
}

// Duration returns the latency as a time.Duration.
func (l Latency) Duration() time.Duration {
	return l.value
}

// IsZero returns true if the latency is zero.
func (l Latency) IsZero() bool {
	return l.value == 0
}

// ExceedsThreshold returns true if this latency exceeds the given threshold.
func (l Latency) ExceedsThreshold(threshold Latency) bool {
	return l.value > threshold.value
}

// SuccessRate represents a success rate between 0.0 and 1.0.
type SuccessRate struct {
	value float64
}

// NewSuccessRate creates a new SuccessRate, clamping to [0.0, 1.0].
func NewSuccessRate(rate float64) SuccessRate {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return SuccessRate{value: rate}
}

// Float64 returns the success rate as a float64.
func (s SuccessRate) Float64() float64 {
	return s.value
}

// BelowThreshold returns true if this rate is below the given threshold.
func (s SuccessRate) BelowThreshold(threshold SuccessRate) bool {
	return s.value < threshold.value
}

// RelayThreshold defines thresholds for determining slow relays.
// All fields are immutable after creation.
type RelayThreshold struct {
	maxLatency     Latency
	minSuccessRate SuccessRate
	blockDuration  time.Duration
	minSamples     int
}

// NewRelayThreshold creates a RelayThreshold with default values.
// Use With* methods to customize.
func NewRelayThreshold() RelayThreshold {
	return RelayThreshold{
		maxLatency:     NewLatency(defaultLatencyThreshold),
		minSuccessRate: NewSuccessRate(defaultMinSuccessRate),
		blockDuration:  defaultBlockDuration,
		minSamples:     defaultMinSamples,
	}
}

// WithMaxLatency returns a new RelayThreshold with the given max latency.
func (t RelayThreshold) WithMaxLatency(d time.Duration) RelayThreshold {
	t.maxLatency = NewLatency(d)
	return t
}

// WithMinSuccessRate returns a new RelayThreshold with the given min success rate.
func (t RelayThreshold) WithMinSuccessRate(rate float64) RelayThreshold {
	t.minSuccessRate = NewSuccessRate(rate)
	return t
}

// WithBlockDuration returns a new RelayThreshold with the given block duration.
func (t RelayThreshold) WithBlockDuration(d time.Duration) RelayThreshold {
	t.blockDuration = d
	return t
}

// WithMinSamples returns a new RelayThreshold with the given minimum sample count.
func (t RelayThreshold) WithMinSamples(n int) RelayThreshold {
	if n < 1 {
		n = 1
	}
	t.minSamples = n
	return t
}

// MaxLatency returns the maximum acceptable latency.
func (t RelayThreshold) MaxLatency() Latency {
	return t.maxLatency
}

// MinSuccessRate returns the minimum acceptable success rate.
func (t RelayThreshold) MinSuccessRate() SuccessRate {
	return t.minSuccessRate
}

// BlockDuration returns how long slow relays are blocked.
func (t RelayThreshold) BlockDuration() time.Duration {
	return t.blockDuration
}

// MinSamples returns the minimum number of samples required for evaluation.
func (t RelayThreshold) MinSamples() int {
	return t.minSamples
}

// RelayMeasurement represents a single performance measurement for a relay.
type RelayMeasurement struct {
	latency   Latency
	success   bool
	timestamp time.Time
}

// NewRelayMeasurement creates a new measurement.
func NewRelayMeasurement(latency time.Duration, success bool) RelayMeasurement {
	return RelayMeasurement{
		latency:   NewLatency(latency),
		success:   success,
		timestamp: time.Now(),
	}
}

// Latency returns the measured latency.
func (m RelayMeasurement) Latency() Latency {
	return m.latency
}

// Success returns whether the measurement was successful.
func (m RelayMeasurement) Success() bool {
	return m.success
}

// Timestamp returns when the measurement was taken.
func (m RelayMeasurement) Timestamp() time.Time {
	return m.timestamp
}

// IsExpired returns true if the measurement is older than the given window.
func (m RelayMeasurement) IsExpired(window time.Duration) bool {
	return time.Since(m.timestamp) > window
}

// RelayStats aggregates performance statistics for a single relay.
type RelayStats struct {
	fingerprint    RelayFingerprint
	measurements   []RelayMeasurement
	mu             sync.RWMutex
	measureWindow  time.Duration
	averageLatency Latency
	successRate    SuccessRate
	sampleCount    int
}

// NewRelayStats creates a new RelayStats for the given fingerprint.
func NewRelayStats(fingerprint RelayFingerprint, measureWindow time.Duration) *RelayStats {
	if measureWindow <= 0 {
		measureWindow = defaultMeasurementWindow
	}
	return &RelayStats{
		fingerprint:   fingerprint,
		measurements:  make([]RelayMeasurement, 0, 16),
		measureWindow: measureWindow,
	}
}

// Fingerprint returns the relay's fingerprint.
func (s *RelayStats) Fingerprint() RelayFingerprint {
	return s.fingerprint
}

// AddMeasurement adds a new measurement and recalculates statistics.
func (s *RelayStats) AddMeasurement(m RelayMeasurement) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.measurements = append(s.measurements, m)
	s.pruneExpired()
	s.recalculate()
}

// pruneExpired removes measurements older than the measurement window.
// Must be called with lock held.
func (s *RelayStats) pruneExpired() {
	valid := make([]RelayMeasurement, 0, len(s.measurements))
	for _, m := range s.measurements {
		if !m.IsExpired(s.measureWindow) {
			valid = append(valid, m)
		}
	}
	s.measurements = valid
}

// recalculate updates the aggregated statistics.
// Must be called with lock held.
func (s *RelayStats) recalculate() {
	s.sampleCount = len(s.measurements)
	if s.sampleCount == 0 {
		s.averageLatency = NewLatency(0)
		s.successRate = NewSuccessRate(0)
		return
	}

	var totalLatency time.Duration
	var successCount int
	for _, m := range s.measurements {
		totalLatency += m.latency.Duration()
		if m.success {
			successCount++
		}
	}

	s.averageLatency = NewLatency(totalLatency / time.Duration(s.sampleCount))
	s.successRate = NewSuccessRate(float64(successCount) / float64(s.sampleCount))
}

// AverageLatency returns the average latency.
func (s *RelayStats) AverageLatency() Latency {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.averageLatency
}

// SuccessRate returns the success rate.
func (s *RelayStats) SuccessRate() SuccessRate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.successRate
}

// SampleCount returns the number of samples in the current window.
func (s *RelayStats) SampleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sampleCount
}

// IsSlow returns true if the relay is considered slow based on the threshold.
func (s *RelayStats) IsSlow(threshold RelayThreshold) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Not enough samples to judge
	if s.sampleCount < threshold.MinSamples() {
		return false
	}

	// Check latency threshold
	if s.averageLatency.ExceedsThreshold(threshold.MaxLatency()) {
		return true
	}

	// Check success rate threshold
	if s.successRate.BelowThreshold(threshold.MinSuccessRate()) {
		return true
	}

	return false
}

// blockedRelay represents a relay that has been blocked.
type blockedRelay struct {
	fingerprint RelayFingerprint
	blockedAt   time.Time
	expiresAt   time.Time
}

// RelayPerformanceTracker tracks relay performance and manages slow relay exclusion.
type RelayPerformanceTracker struct {
	stats         map[string]*RelayStats
	blockedRelays map[string]blockedRelay
	threshold     RelayThreshold
	measureWindow time.Duration
	mu            sync.RWMutex
	logger        Logger
	control       *ControlClient
	autoExclude   bool
}

// RelayTrackerOption configures a RelayPerformanceTracker.
type RelayTrackerOption func(*RelayPerformanceTracker)

// NewRelayPerformanceTracker creates a new tracker with optional configuration.
func NewRelayPerformanceTracker(opts ...RelayTrackerOption) *RelayPerformanceTracker {
	t := &RelayPerformanceTracker{
		stats:         make(map[string]*RelayStats),
		blockedRelays: make(map[string]blockedRelay),
		threshold:     NewRelayThreshold(),
		measureWindow: defaultMeasurementWindow,
		logger:        noopLogger{},
		autoExclude:   true,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// WithTrackerThreshold sets the threshold configuration.
func WithTrackerThreshold(threshold RelayThreshold) RelayTrackerOption {
	return func(t *RelayPerformanceTracker) {
		t.threshold = threshold
	}
}

// WithTrackerMeasureWindow sets the measurement window.
func WithTrackerMeasureWindow(d time.Duration) RelayTrackerOption {
	return func(t *RelayPerformanceTracker) {
		if d > 0 {
			t.measureWindow = d
		}
	}
}

// WithTrackerLogger sets the logger.
func WithTrackerLogger(logger Logger) RelayTrackerOption {
	return func(t *RelayPerformanceTracker) {
		t.logger = logger
	}
}

// WithTrackerControl sets the control client for automatic ExcludeNodes.
func WithTrackerControl(control *ControlClient) RelayTrackerOption {
	return func(t *RelayPerformanceTracker) {
		t.control = control
	}
}

// WithTrackerAutoExclude enables/disables automatic relay exclusion via Tor.
func WithTrackerAutoExclude(enabled bool) RelayTrackerOption {
	return func(t *RelayPerformanceTracker) {
		t.autoExclude = enabled
	}
}

// RecordMeasurement records a performance measurement for a relay in a circuit.
func (t *RelayPerformanceTracker) RecordMeasurement(fingerprint RelayFingerprint, latency time.Duration, success bool) {
	if fingerprint.IsEmpty() {
		return
	}

	t.mu.Lock()
	stats, exists := t.stats[fingerprint.String()]
	if !exists {
		stats = NewRelayStats(fingerprint, t.measureWindow)
		t.stats[fingerprint.String()] = stats
	}
	t.mu.Unlock()

	measurement := NewRelayMeasurement(latency, success)
	stats.AddMeasurement(measurement)

	// Check if relay should be blocked
	if stats.IsSlow(t.threshold) {
		t.blockRelay(fingerprint)
	}
}

// RecordCircuitMeasurement records a measurement for all relays in a circuit path.
func (t *RelayPerformanceTracker) RecordCircuitMeasurement(path []string, latency time.Duration, success bool) {
	for _, fp := range path {
		fingerprint := NewRelayFingerprint(fp)
		t.RecordMeasurement(fingerprint, latency, success)
	}
}

// blockRelay adds a relay to the block list.
func (t *RelayPerformanceTracker) blockRelay(fingerprint RelayFingerprint) {
	t.mu.Lock()
	defer t.mu.Unlock()

	fpStr := fingerprint.String()
	if _, alreadyBlocked := t.blockedRelays[fpStr]; alreadyBlocked {
		return
	}

	now := time.Now()
	t.blockedRelays[fpStr] = blockedRelay{
		fingerprint: fingerprint,
		blockedAt:   now,
		expiresAt:   now.Add(t.threshold.BlockDuration()),
	}

	t.logger.Log("info", "blocking slow relay",
		"fingerprint", fpStr,
		"duration", t.threshold.BlockDuration())

	// Apply ExcludeNodes if control client is available
	if t.control != nil && t.autoExclude {
		go t.applyExcludeNodes()
	}
}

// applyExcludeNodes updates Tor's ExcludeNodes configuration.
func (t *RelayPerformanceTracker) applyExcludeNodes() {
	t.mu.RLock()
	blocked := t.getBlockedFingerprints()
	t.mu.RUnlock()

	if len(blocked) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	nodeList := strings.Join(blocked, ",")
	if err := t.control.SetConf(ctx, "ExcludeNodes", nodeList); err != nil {
		t.logger.Log("error", "failed to set ExcludeNodes", "error", err)
	} else {
		t.logger.Log("info", "updated ExcludeNodes", "count", len(blocked))
	}
}

// getBlockedFingerprints returns currently blocked fingerprints.
// Must be called with at least read lock held.
func (t *RelayPerformanceTracker) getBlockedFingerprints() []string {
	now := time.Now()
	var blocked []string
	for fp, br := range t.blockedRelays {
		if now.Before(br.expiresAt) {
			blocked = append(blocked, "$"+fp)
		}
	}
	return blocked
}

// BlockedRelays returns a list of currently blocked relay fingerprints.
func (t *RelayPerformanceTracker) BlockedRelays() []RelayFingerprint {
	t.mu.RLock()
	defer t.mu.RUnlock()

	t.pruneExpiredBlocks()

	relays := make([]RelayFingerprint, 0, len(t.blockedRelays))
	for _, br := range t.blockedRelays {
		relays = append(relays, br.fingerprint)
	}
	return relays
}

// pruneExpiredBlocks removes expired blocks.
// Must be called with lock held.
func (t *RelayPerformanceTracker) pruneExpiredBlocks() {
	now := time.Now()
	for fp, br := range t.blockedRelays {
		if now.After(br.expiresAt) {
			delete(t.blockedRelays, fp)
		}
	}
}

// IsBlocked returns true if the relay is currently blocked.
func (t *RelayPerformanceTracker) IsBlocked(fingerprint RelayFingerprint) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	br, exists := t.blockedRelays[fingerprint.String()]
	if !exists {
		return false
	}
	return time.Now().Before(br.expiresAt)
}

// GetStats returns statistics for a specific relay.
func (t *RelayPerformanceTracker) GetStats(fingerprint RelayFingerprint) *RelayStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats[fingerprint.String()]
}

// Clear removes all tracked statistics and blocks.
func (t *RelayPerformanceTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats = make(map[string]*RelayStats)
	t.blockedRelays = make(map[string]blockedRelay)
}

// ClearExcludeNodes removes all ExcludeNodes from Tor configuration.
func (t *RelayPerformanceTracker) ClearExcludeNodes(ctx context.Context) error {
	if t.control == nil {
		return nil
	}
	return t.control.ResetConf(ctx, "ExcludeNodes")
}

// TrackerStats provides statistics about the tracker.
type TrackerStats struct {
	// TrackedRelays is the number of relays being tracked.
	TrackedRelays int
	// BlockedRelays is the number of currently blocked relays.
	BlockedRelays int
	// Threshold is the current threshold configuration.
	Threshold RelayThreshold
}

// Stats returns current tracker statistics.
func (t *RelayPerformanceTracker) Stats() TrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	t.pruneExpiredBlocks()

	return TrackerStats{
		TrackedRelays: len(t.stats),
		BlockedRelays: len(t.blockedRelays),
		Threshold:     t.threshold,
	}
}

// slowRelayAvoidanceManager encapsulates all slow relay avoidance functionality
// for use within Client. This is an internal type that provides a simple interface.
type slowRelayAvoidanceManager struct {
	tracker       *RelayPerformanceTracker
	control       *ControlClient
	logger        Logger
	interval      time.Duration
	monitorStopCh chan struct{}
	running       bool
	mu            sync.Mutex
}

// SlowRelayOption configures slow relay avoidance behavior.
type SlowRelayOption func(*slowRelayConfig)

// slowRelayConfig holds configuration for slow relay avoidance.
type slowRelayConfig struct {
	maxLatency      time.Duration
	minSuccessRate  float64
	blockDuration   time.Duration
	minSamples      int
	measureWindow   time.Duration
	monitorInterval time.Duration
	autoExclude     bool
}

// newSlowRelayConfig creates a config with default values.
func newSlowRelayConfig() *slowRelayConfig {
	return &slowRelayConfig{
		maxLatency:      defaultLatencyThreshold,
		minSuccessRate:  defaultMinSuccessRate,
		blockDuration:   defaultBlockDuration,
		minSamples:      defaultMinSamples,
		measureWindow:   defaultMeasurementWindow,
		monitorInterval: 30 * time.Second,
		autoExclude:     true,
	}
}

// SlowRelayMaxLatency sets the maximum acceptable latency.
// Relays with average latency exceeding this will be blocked.
// Default: 5 seconds.
func SlowRelayMaxLatency(d time.Duration) SlowRelayOption {
	return func(c *slowRelayConfig) {
		if d > 0 {
			c.maxLatency = d
		}
	}
}

// SlowRelayMinSuccessRate sets the minimum acceptable success rate (0.0-1.0).
// Relays with lower success rates will be blocked.
// Default: 0.8 (80%).
func SlowRelayMinSuccessRate(rate float64) SlowRelayOption {
	return func(c *slowRelayConfig) {
		if rate >= 0 && rate <= 1 {
			c.minSuccessRate = rate
		}
	}
}

// SlowRelayBlockDuration sets how long slow relays remain blocked.
// Default: 30 minutes.
func SlowRelayBlockDuration(d time.Duration) SlowRelayOption {
	return func(c *slowRelayConfig) {
		if d > 0 {
			c.blockDuration = d
		}
	}
}

// SlowRelayMinSamples sets the minimum measurements needed before evaluation.
// Default: 3.
func SlowRelayMinSamples(n int) SlowRelayOption {
	return func(c *slowRelayConfig) {
		if n > 0 {
			c.minSamples = n
		}
	}
}

// SlowRelayMonitorInterval sets how often to check for slow circuits.
// Default: 30 seconds.
func SlowRelayMonitorInterval(d time.Duration) SlowRelayOption {
	return func(c *slowRelayConfig) {
		if d > 0 {
			c.monitorInterval = d
		}
	}
}

// SlowRelayAutoExclude enables/disables automatic Tor ExcludeNodes updates.
// When enabled, blocked relays are added to Tor's ExcludeNodes configuration.
// Default: true.
func SlowRelayAutoExclude(enabled bool) SlowRelayOption {
	return func(c *slowRelayConfig) {
		c.autoExclude = enabled
	}
}

// newSlowRelayAvoidanceManager creates a new manager with the given options.
func newSlowRelayAvoidanceManager(control *ControlClient, logger Logger, opts ...SlowRelayOption) *slowRelayAvoidanceManager {
	cfg := newSlowRelayConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	threshold := NewRelayThreshold().
		WithMaxLatency(cfg.maxLatency).
		WithMinSuccessRate(cfg.minSuccessRate).
		WithBlockDuration(cfg.blockDuration).
		WithMinSamples(cfg.minSamples)

	tracker := NewRelayPerformanceTracker(
		WithTrackerThreshold(threshold),
		WithTrackerMeasureWindow(cfg.measureWindow),
		WithTrackerLogger(logger),
		WithTrackerControl(control),
		WithTrackerAutoExclude(cfg.autoExclude),
	)

	return &slowRelayAvoidanceManager{
		tracker:       tracker,
		control:       control,
		logger:        logger,
		interval:      cfg.monitorInterval,
		monitorStopCh: make(chan struct{}),
	}
}

// monitorInterval returns the configured monitor interval.
func (m *slowRelayAvoidanceManager) monitorInterval() time.Duration {
	return m.interval
}

// start begins the background monitoring goroutine.
func (m *slowRelayAvoidanceManager) start(interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running || m.control == nil {
		return
	}

	m.monitorStopCh = make(chan struct{})
	m.running = true

	go m.monitorLoop(interval)
	m.logger.Log("info", "slow relay avoidance started", "interval", interval)
}

// stop stops the background monitoring goroutine.
func (m *slowRelayAvoidanceManager) stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.monitorStopCh)
	m.running = false
	m.logger.Log("info", "slow relay avoidance stopped")
}

// monitorLoop periodically checks circuits and rotates if slow relays are detected.
func (m *slowRelayAvoidanceManager) monitorLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.monitorStopCh:
			return
		case <-ticker.C:
			m.checkAndRotate()
		}
	}
}

// checkAndRotate checks current circuits and rotates if any use blocked relays.
func (m *slowRelayAvoidanceManager) checkAndRotate() {
	if m.control == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	circuits, err := m.control.GetCircuitStatus(ctx)
	if err != nil {
		m.logger.Log("debug", "failed to get circuit status for slow relay check", "error", err)
		return
	}

	for _, circuit := range circuits {
		if circuit.Status != CircuitStatusBuilt {
			continue
		}

		for _, relay := range circuit.Path {
			fp := NewRelayFingerprint(relay)
			if m.tracker.IsBlocked(fp) {
				m.logger.Log("info", "rotating circuit due to slow relay",
					"circuit_id", circuit.ID,
					"relay", fp.String())

				if err := m.control.NewIdentity(ctx); err != nil {
					m.logger.Log("error", "failed to rotate circuit", "error", err)
				}
				return // One rotation per check is enough
			}
		}
	}
}

// recordMeasurement records a latency measurement and triggers rotation if needed.
func (m *slowRelayAvoidanceManager) recordMeasurement(latency time.Duration, success bool) {
	if m.control == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	circuits, err := m.control.GetCircuitStatus(ctx)
	if err != nil {
		return
	}

	// Find an active built circuit and record measurement for its relays
	for _, circuit := range circuits {
		if circuit.Status == CircuitStatusBuilt && len(circuit.Path) > 0 {
			m.tracker.RecordCircuitMeasurement(circuit.Path, latency, success)
			return
		}
	}
}

// RelayPerformanceStats provides detailed statistics about relay performance.
// This is an immutable Value Object - use accessor methods to retrieve values.
type RelayPerformanceStats struct {
	enabled          bool
	trackedRelays    int
	blockedRelays    int
	blockedRelayList []string
	threshold        RelayThresholdStats
}

// Enabled returns true if slow relay avoidance is active.
func (s RelayPerformanceStats) Enabled() bool { return s.enabled }

// TrackedRelays returns the number of relays being monitored.
func (s RelayPerformanceStats) TrackedRelays() int { return s.trackedRelays }

// BlockedRelays returns the number of currently blocked relays.
func (s RelayPerformanceStats) BlockedRelays() int { return s.blockedRelays }

// BlockedRelayList returns a copy of the blocked relay fingerprints.
func (s RelayPerformanceStats) BlockedRelayList() []string {
	if s.blockedRelayList == nil {
		return nil
	}
	result := make([]string, len(s.blockedRelayList))
	copy(result, s.blockedRelayList)
	return result
}

// Threshold returns the current threshold configuration.
func (s RelayPerformanceStats) Threshold() RelayThresholdStats { return s.threshold }

// RelayThresholdStats provides threshold configuration details.
// This is an immutable Value Object - use accessor methods to retrieve values.
type RelayThresholdStats struct {
	maxLatency     time.Duration
	minSuccessRate float64
	blockDuration  time.Duration
	minSamples     int
}

// MaxLatency returns the maximum acceptable latency.
func (s RelayThresholdStats) MaxLatency() time.Duration { return s.maxLatency }

// MinSuccessRate returns the minimum acceptable success rate.
func (s RelayThresholdStats) MinSuccessRate() float64 { return s.minSuccessRate }

// BlockDuration returns how long slow relays remain blocked.
func (s RelayThresholdStats) BlockDuration() time.Duration { return s.blockDuration }

// MinSamples returns the minimum measurements needed before evaluation.
func (s RelayThresholdStats) MinSamples() int { return s.minSamples }

// stats returns current performance statistics.
func (m *slowRelayAvoidanceManager) stats() RelayPerformanceStats {
	if m == nil || m.tracker == nil {
		return RelayPerformanceStats{enabled: false}
	}

	trackerStats := m.tracker.Stats()
	blocked := m.tracker.BlockedRelays()
	blockedList := make([]string, len(blocked))
	for i, fp := range blocked {
		blockedList[i] = fp.String()
	}

	return RelayPerformanceStats{
		enabled:          true,
		trackedRelays:    trackerStats.TrackedRelays,
		blockedRelays:    trackerStats.BlockedRelays,
		blockedRelayList: blockedList,
		threshold: RelayThresholdStats{
			maxLatency:     trackerStats.Threshold.MaxLatency().Duration(),
			minSuccessRate: trackerStats.Threshold.MinSuccessRate().Float64(),
			blockDuration:  trackerStats.Threshold.BlockDuration(),
			minSamples:     trackerStats.Threshold.MinSamples(),
		},
	}
}
