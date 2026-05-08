package router

import "testing"

func TestHealthTransitions(t *testing.T) {
	h := NewHealth()
	if h.State != HealthHealthy {
		t.Fatalf("state = %s", h.State)
	}
	h.Failure()
	if h.State != HealthDegraded {
		t.Fatalf("state after failure = %s", h.State)
	}
	h.Failure()
	h.Failure()
	if h.State != HealthSick {
		t.Fatalf("state after failures = %s", h.State)
	}
	h.Cooldown()
	if h.Available() {
		t.Fatalf("cooling off should not be available during normal selection")
	}
	h.Success()
	if h.State != HealthHealthy {
		t.Fatalf("state after success = %s", h.State)
	}
}
