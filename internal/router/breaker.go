package router

import "time"

type BreakerState string

const (
	BreakerClosed   BreakerState = "closed"
	BreakerOpen     BreakerState = "open"
	BreakerHalfOpen BreakerState = "half_open"
)

type Breaker struct {
	State           BreakerState
	Failures        int
	Threshold       int
	RecoveryTimeout time.Duration
	openedAt        time.Time
	now             func() time.Time
}

func NewBreaker(threshold int, recoveryTimeout time.Duration) *Breaker {
	if threshold <= 0 {
		threshold = 3
	}
	if recoveryTimeout <= 0 {
		recoveryTimeout = time.Second
	}
	return &Breaker{State: BreakerClosed, Threshold: threshold, RecoveryTimeout: recoveryTimeout, now: time.Now}
}

func (b *Breaker) Allow() bool {
	if b.State != BreakerOpen {
		return true
	}
	if b.now().Sub(b.openedAt) >= b.RecoveryTimeout {
		b.State = BreakerHalfOpen
		return true
	}
	return false
}

func (b *Breaker) Success() {
	b.Failures = 0
	b.State = BreakerClosed
}

func (b *Breaker) Failure() {
	b.Failures++
	if b.State == BreakerHalfOpen || b.Failures >= b.Threshold {
		b.State = BreakerOpen
		b.openedAt = b.now()
	}
}
