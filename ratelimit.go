package tornago

import (
	"context"
	"sync"
	"time"
)

// RateLimiter controls the rate of requests to prevent overloading Tor circuits.
// It implements a token bucket algorithm.
type RateLimiter struct {
	// rate is the number of requests allowed per second.
	rate float64
	// burst is the maximum number of requests that can be made at once.
	burst int
	// tokens is the current number of available tokens.
	tokens float64
	// lastUpdate is when tokens were last replenished.
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a rate limiter with the specified rate (requests/second) and burst size.
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

// Wait blocks until a token is available or the context is canceled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(r.lastUpdate).Seconds()
		r.tokens += elapsed * r.rate
		if r.tokens > float64(r.burst) {
			r.tokens = float64(r.burst)
		}
		r.lastUpdate = now

		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time for next token
		waitDuration := time.Duration((1 - r.tokens) / r.rate * float64(time.Second))
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// Continue and try again
		}
	}
}

// Allow returns true if a request can proceed immediately without waiting.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.tokens += elapsed * r.rate
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
	r.lastUpdate = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Rate returns the configured rate (requests per second).
func (r *RateLimiter) Rate() float64 {
	return r.rate
}

// Burst returns the configured burst size.
func (r *RateLimiter) Burst() int {
	return r.burst
}
