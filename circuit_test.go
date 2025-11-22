package tornago

import (
	"context"
	"testing"
	"time"
)

// TestCircuitManager runs all circuit manager tests as subtests with a single Tor instance.
func TestCircuitManager(t *testing.T) {
	// Use shared global test server
	ts := getGlobalTestServer(t)

	// Helper to create a fresh ControlClient for each test to avoid connection state issues
	newFreshControl := func(t *testing.T) *ControlClient {
		t.Helper()
		auth := ts.ControlAuth(t)
		ctrl, err := NewControlClient(ts.Server.ControlAddr(), auth, 30*time.Second)
		if err != nil {
			t.Fatalf("NewControlClient: %v", err)
		}
		return ctrl
	}

	// Run all subtests
	t.Run("NewCircuitManager", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		if manager == nil {
			t.Fatal("NewCircuitManager() returned nil")
		}

		if manager.IsRunning() {
			t.Error("NewCircuitManager() should not be running initially")
		}
	})

	t.Run("WithLogger", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		logger := noopLogger{}
		manager := NewCircuitManager(ctrl).WithLogger(logger)

		if manager == nil {
			t.Fatal("WithLogger() returned nil")
		}
	})

	t.Run("StartAutoRotation", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Start auto-rotation
		err := manager.StartAutoRotation(ctx, 5*time.Second)
		if err != nil {
			t.Fatalf("StartAutoRotation() error = %v", err)
		}

		if !manager.IsRunning() {
			t.Error("manager should be running after StartAutoRotation()")
		}

		// Wait for at least one rotation
		time.Sleep(6 * time.Second)

		// Stop manager
		manager.Stop()

		// Give it time to actually stop
		time.Sleep(100 * time.Millisecond)

		if manager.IsRunning() {
			t.Error("manager should not be running after Stop()")
		}
	})

	t.Run("StartAutoRotation_InvalidInterval", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		ctx := context.Background()

		// Try to start with invalid interval
		err := manager.StartAutoRotation(ctx, 0)
		if err == nil {
			t.Error("StartAutoRotation() with zero interval should return error")
		}

		err = manager.StartAutoRotation(ctx, -1*time.Second)
		if err == nil {
			t.Error("StartAutoRotation() with negative interval should return error")
		}
	})

	t.Run("StartAutoRotation_AlreadyRunning", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start first time
		err := manager.StartAutoRotation(ctx, 5*time.Second)
		if err != nil {
			t.Fatalf("first StartAutoRotation() error = %v", err)
		}
		defer manager.Stop()

		// Try to start again
		err = manager.StartAutoRotation(ctx, 5*time.Second)
		if err == nil {
			t.Error("second StartAutoRotation() should return error")
		}
	})

	t.Run("RotateNow", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Rotate circuits manually
		err := manager.RotateNow(ctx)
		if err != nil {
			t.Errorf("RotateNow() error = %v", err)
		}
	})

	t.Run("PrewarmCircuits", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Prewarm circuits
		err := manager.PrewarmCircuits(ctx)
		if err != nil {
			t.Errorf("PrewarmCircuits() error = %v", err)
		}
	})

	t.Run("Stats", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)

		// Initial stats
		stats := manager.Stats()
		if stats.AutoRotationEnabled {
			t.Error("AutoRotationEnabled should be false initially")
		}
		if stats.RotationInterval != 0 {
			t.Errorf("RotationInterval should be 0 initially, got %v", stats.RotationInterval)
		}

		// Start auto-rotation
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		interval := 5 * time.Second
		err := manager.StartAutoRotation(ctx, interval)
		if err != nil {
			t.Fatalf("StartAutoRotation() error = %v", err)
		}
		defer manager.Stop()

		// Stats after starting
		stats = manager.Stats()
		if !stats.AutoRotationEnabled {
			t.Error("AutoRotationEnabled should be true after StartAutoRotation()")
		}
		if stats.RotationInterval != interval {
			t.Errorf("RotationInterval = %v, want %v", stats.RotationInterval, interval)
		}
	})

	t.Run("Stop_NotRunning", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)

		// Stopping when not running should not panic
		manager.Stop()

		if manager.IsRunning() {
			t.Error("manager should not be running")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctrl := newFreshControl(t)
		defer ctrl.Close()
		manager := NewCircuitManager(ctrl)
		ctx, cancel := context.WithCancel(context.Background())

		// Start auto-rotation
		err := manager.StartAutoRotation(ctx, 10*time.Second)
		if err != nil {
			t.Fatalf("StartAutoRotation() error = %v", err)
		}

		if !manager.IsRunning() {
			t.Fatal("manager should be running")
		}

		// Cancel context
		cancel()

		// Wait for manager to stop
		time.Sleep(200 * time.Millisecond)

		if manager.IsRunning() {
			t.Error("manager should have stopped after context cancellation")
		}
	})
}
