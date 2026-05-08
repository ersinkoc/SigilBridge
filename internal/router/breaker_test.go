package router

import (
	"testing"
	"time"
)

func TestBreakerTransitions(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	breaker := NewBreaker(2, time.Second)
	breaker.now = func() time.Time { return now }
	breaker.Failure()
	if breaker.State != BreakerClosed {
		t.Fatalf("state = %s", breaker.State)
	}
	breaker.Failure()
	if breaker.State != BreakerOpen || breaker.Allow() {
		t.Fatalf("state=%s allow=%v", breaker.State, breaker.Allow())
	}
	now = now.Add(2 * time.Second)
	if !breaker.Allow() || breaker.State != BreakerHalfOpen {
		t.Fatalf("state=%s", breaker.State)
	}
	breaker.Success()
	if breaker.State != BreakerClosed {
		t.Fatalf("state=%s", breaker.State)
	}
}
