package router

type HealthState string

const (
	HealthHealthy    HealthState = "healthy"
	HealthDegraded   HealthState = "degraded"
	HealthSick       HealthState = "sick"
	HealthCoolingOff HealthState = "cooling_off"
)

type Health struct {
	State            HealthState
	ConsecutiveOK    int
	ConsecutiveError int
}

func NewHealth() Health {
	return Health{State: HealthHealthy}
}

func (h *Health) Success() {
	h.ConsecutiveOK++
	h.ConsecutiveError = 0
	if h.ConsecutiveOK >= 1 {
		h.State = HealthHealthy
	}
}

func (h *Health) Failure() {
	h.ConsecutiveError++
	h.ConsecutiveOK = 0
	switch {
	case h.ConsecutiveError >= 3:
		h.State = HealthSick
	case h.ConsecutiveError >= 1:
		h.State = HealthDegraded
	}
}

func (h *Health) Cooldown() {
	h.State = HealthCoolingOff
}

func (h Health) Available() bool {
	return h.State == HealthHealthy || h.State == HealthDegraded
}
