package tornago

import (
	"context"
	"sync"
	"time"
)

const (
	// opCircuitManager labels errors originating from CircuitManager operations.
	opCircuitManager = "CircuitManager"
)

// CircuitManager manages Tor circuits with advanced features like automatic rotation,
// circuit prewarming, and per-site circuit isolation.
//
// Circuit management is useful for:
//   - Automatic IP rotation on a schedule
//   - Pre-building circuits before they're needed (prewarming)
//   - Isolating circuits for specific destinations (privacy)
//   - Recovering from circuit failures
//
// Example usage:
//
//	manager := tornago.NewCircuitManager(controlClient)
//	manager.StartAutoRotation(ctx, 10*time.Minute)  // Rotate every 10 minutes
//	defer manager.Stop()
type CircuitManager struct {
	// control is the ControlClient used for circuit operations.
	control *ControlClient
	// logger for circuit management operations.
	logger Logger
	// rotationInterval is how often to rotate circuits automatically.
	rotationInterval time.Duration
	// rotationTimer triggers automatic circuit rotation.
	rotationTimer *time.Timer
	// stopCh signals the manager to stop.
	stopCh chan struct{}
	// mu protects concurrent access to manager state.
	mu sync.Mutex
	// running indicates if auto-rotation is active.
	running bool
}

// NewCircuitManager creates a new CircuitManager with the given ControlClient.
func NewCircuitManager(control *ControlClient) *CircuitManager {
	return &CircuitManager{
		control: control,
		logger:  noopLogger{},
		stopCh:  make(chan struct{}),
	}
}

// WithLogger sets a logger for circuit management operations.
func (m *CircuitManager) WithLogger(logger Logger) *CircuitManager {
	m.logger = logger
	return m
}

// StartAutoRotation begins automatic circuit rotation at the specified interval.
// Circuits will be rotated by calling NewIdentity() at regular intervals.
//
// This is useful for:
//   - Changing exit IPs periodically for privacy
//   - Avoiding rate limiting by rotating IPs
//   - Refreshing circuits that may have become slow
//
// The rotation continues until Stop() is called or the context is canceled.
//
// Example:
//
//	manager.StartAutoRotation(ctx, 10*time.Minute)
//	// Circuits rotate every 10 minutes
func (m *CircuitManager) StartAutoRotation(ctx context.Context, interval time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return newError(ErrInvalidConfig, opCircuitManager, "auto-rotation already running", nil)
	}

	if interval <= 0 {
		return newError(ErrInvalidConfig, opCircuitManager, "rotation interval must be positive", nil)
	}

	m.rotationInterval = interval
	m.running = true

	m.logger.Log("info", "starting auto-rotation", "interval", interval)

	// Start rotation goroutine
	go m.autoRotateLoop(ctx)

	return nil
}

// autoRotateLoop runs the automatic rotation logic.
func (m *CircuitManager) autoRotateLoop(ctx context.Context) {
	m.rotationTimer = time.NewTimer(m.rotationInterval)
	defer m.rotationTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Log("info", "auto-rotation stopped", "reason", "context canceled")
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			return

		case <-m.stopCh:
			m.logger.Log("info", "auto-rotation stopped", "reason", "stop requested")
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			return

		case <-m.rotationTimer.C:
			m.logger.Log("debug", "rotating circuits", "interval", m.rotationInterval)

			if err := m.control.NewIdentity(ctx); err != nil {
				m.logger.Log("error", "circuit rotation failed", "error", err)
			} else {
				m.logger.Log("info", "circuits rotated successfully")
			}

			// Reset timer for next rotation
			m.rotationTimer.Reset(m.rotationInterval)
		}
	}
}

// Stop stops automatic circuit rotation if it's running.
func (m *CircuitManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.logger.Log("info", "stopping circuit manager")
	close(m.stopCh)
	m.running = false
}

// IsRunning returns true if automatic rotation is currently active.
func (m *CircuitManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// RotateNow immediately rotates circuits by calling NewIdentity().
// This is useful for manual circuit rotation outside of the automatic schedule.
func (m *CircuitManager) RotateNow(ctx context.Context) error {
	m.logger.Log("debug", "manual circuit rotation requested")

	if err := m.control.NewIdentity(ctx); err != nil {
		m.logger.Log("error", "manual circuit rotation failed", "error", err)
		return err
	}

	m.logger.Log("info", "manual circuit rotation completed")
	return nil
}

// PrewarmCircuits builds new circuits in advance to reduce latency for future requests.
// This calls NewIdentity() to signal Tor to build fresh circuits.
//
// Prewarming is useful before:
//   - Starting a batch of requests
//   - After a long idle period
//   - When you know you'll need fresh circuits soon
//
// After calling this, wait a few seconds (5-10s) for Tor to build new circuits
// before making requests.
func (m *CircuitManager) PrewarmCircuits(ctx context.Context) error {
	m.logger.Log("info", "prewarming circuits")

	if err := m.control.NewIdentity(ctx); err != nil {
		m.logger.Log("error", "circuit prewarming failed", "error", err)
		return err
	}

	m.logger.Log("info", "circuit prewarming initiated", "wait_time", "5-10 seconds recommended")
	return nil
}

// CircuitStats provides statistics about circuit management operations.
type CircuitStats struct {
	// AutoRotationEnabled indicates if automatic rotation is running.
	AutoRotationEnabled bool
	// RotationInterval is the configured rotation interval (0 if not running).
	RotationInterval time.Duration
}

// Stats returns current statistics about circuit management.
func (m *CircuitManager) Stats() CircuitStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	return CircuitStats{
		AutoRotationEnabled: m.running,
		RotationInterval:    m.rotationInterval,
	}
}
