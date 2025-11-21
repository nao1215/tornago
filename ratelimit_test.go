package tornago

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_Initial(t *testing.T) {
	r := NewRateLimiter(10, 5)
	if r.Rate() != 10 {
		t.Errorf("expected rate 10, got %f", r.Rate())
	}
	if r.Burst() != 5 {
		t.Errorf("expected burst 5, got %d", r.Burst())
	}
}

func TestRateLimiter_InvalidValues(t *testing.T) {
	r := NewRateLimiter(0, 0)
	if r.Rate() != 1 {
		t.Errorf("expected rate 1 for invalid input, got %f", r.Rate())
	}
	if r.Burst() != 1 {
		t.Errorf("expected burst 1 for invalid input, got %d", r.Burst())
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	r := NewRateLimiter(100, 3)

	// Should allow burst requests
	for i := range 3 {
		if !r.Allow() {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// Fourth request should be denied (no tokens left)
	if r.Allow() {
		t.Error("fourth request should be denied")
	}
}

func TestRateLimiter_WaitContext(t *testing.T) {
	r := NewRateLimiter(100, 1)

	ctx := context.Background()
	if err := r.Wait(ctx); err != nil {
		t.Errorf("first wait should succeed: %v", err)
	}
}

func TestRateLimiter_WaitContextCanceled(t *testing.T) {
	r := NewRateLimiter(0.1, 1) // Very slow rate

	// Consume the only token
	r.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := r.Wait(ctx)
	if err == nil {
		t.Error("expected context deadline exceeded")
	}
}

func TestRateLimiter_TokenReplenish(t *testing.T) {
	r := NewRateLimiter(1000, 2) // 1000 tokens/sec

	// Consume all tokens
	r.Allow()
	r.Allow()

	// Wait a bit for token replenish
	time.Sleep(5 * time.Millisecond)

	// Should have some tokens now
	if !r.Allow() {
		t.Error("tokens should have replenished")
	}
}
