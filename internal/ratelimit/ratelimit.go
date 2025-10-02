package ratelimit

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
	SetDelay(min, max time.Duration)
}

type SimpleRateLimiter struct {
	minDelay   time.Duration
	maxDelay   time.Duration
	lastAction time.Time
	mu         sync.Mutex
	jitter     bool
}

func NewSimpleRateLimiter(minDelay, maxDelay time.Duration) *SimpleRateLimiter {
	return &SimpleRateLimiter{
		minDelay: minDelay,
		maxDelay: maxDelay,
		jitter:   true,
	}
}

func (r *SimpleRateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	elapsed := time.Since(r.lastAction)
	delay := r.calculateDelay()
	
	if elapsed < delay {
		waitTime := delay - elapsed
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}
	
	r.lastAction = time.Now()
	return nil
}

func (r *SimpleRateLimiter) SetDelay(min, max time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.minDelay = min
	r.maxDelay = max
}

func (r *SimpleRateLimiter) calculateDelay() time.Duration {
	if !r.jitter || r.minDelay == r.maxDelay {
		return r.minDelay
	}
	
	delta := r.maxDelay - r.minDelay
	jitter := time.Duration(rand.Int63n(int64(delta)))
	return r.minDelay + jitter
}

type AdaptiveRateLimiter struct {
	*SimpleRateLimiter
	errorCount    int
	successCount  int
	maxErrorCount int
	backoffFactor float64
}

func NewAdaptiveRateLimiter(minDelay, maxDelay time.Duration) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		SimpleRateLimiter: NewSimpleRateLimiter(minDelay, maxDelay),
		maxErrorCount:     3,
		backoffFactor:     1.5,
	}
}

func (a *AdaptiveRateLimiter) RecordSuccess() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.successCount++
	a.errorCount = 0
	
	if a.successCount > 5 {
		newMin := time.Duration(float64(a.minDelay) * 0.9)
		if newMin < 1*time.Second {
			newMin = 1 * time.Second
		}
		a.minDelay = newMin
		a.successCount = 0
	}
}

func (a *AdaptiveRateLimiter) RecordError() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.errorCount++
	a.successCount = 0
	
	if a.errorCount >= a.maxErrorCount {
		newMin := time.Duration(float64(a.minDelay) * a.backoffFactor)
		newMax := time.Duration(float64(a.maxDelay) * a.backoffFactor)
		
		if newMin > 60*time.Second {
			newMin = 60 * time.Second
		}
		if newMax > 120*time.Second {
			newMax = 120 * time.Second
		}
		
		a.minDelay = newMin
		a.maxDelay = newMax
		a.errorCount = 0
	}
}

type TokenBucketRateLimiter struct {
	tokens       int
	maxTokens    int
	refillRate   time.Duration
	lastRefill   time.Time
	mu           sync.Mutex
	minDelay     time.Duration
}

func NewTokenBucketRateLimiter(maxTokens int, refillRate time.Duration) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
		minDelay:   1 * time.Second,
	}
}

func (t *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.refill()
	
	for t.tokens <= 0 {
		t.mu.Unlock()
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(t.refillRate):
		}
		
		t.mu.Lock()
		t.refill()
	}
	
	t.tokens--
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(t.minDelay):
		return nil
	}
}

func (t *TokenBucketRateLimiter) refill() {
	elapsed := time.Since(t.lastRefill)
	tokensToAdd := int(elapsed / t.refillRate)
	
	if tokensToAdd > 0 {
		t.tokens += tokensToAdd
		if t.tokens > t.maxTokens {
			t.tokens = t.maxTokens
		}
		t.lastRefill = time.Now()
	}
}

func (t *TokenBucketRateLimiter) SetDelay(min, max time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.minDelay = min
}