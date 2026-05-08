package observability

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registryOnce sync.Once
	registry     *prometheus.Registry
)

func Registry() *prometheus.Registry {
	registryOnce.Do(func() {
		registry = prometheus.NewRegistry()
	})
	return registry
}

type Metrics struct {
	RequestsTotal       *prometheus.CounterVec
	TokensTotal         *prometheus.CounterVec
	CostCentsTotal      *prometheus.CounterVec
	ErrorsTotal         *prometheus.CounterVec
	RequestDuration     *prometheus.HistogramVec
	TimeToFirstByte     *prometheus.HistogramVec
	InflightRequests    *prometheus.GaugeVec
	UpstreamHealth      *prometheus.GaugeVec
	CircuitBreakerState *prometheus.GaugeVec
	BudgetUsedCents     *prometheus.GaugeVec
	BudgetLimitCents    *prometheus.GaugeVec
	SessionExpiry       *prometheus.GaugeVec
}

func NewMetrics(reg *prometheus.Registry) (*Metrics, error) {
	if reg == nil {
		reg = Registry()
	}

	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigilbridge_requests_total",
			Help: "Total requests handled by SigilBridge.",
		}, []string{"ingress", "model", "provider", "upstream", "status"}),
		TokensTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigilbridge_tokens_total",
			Help: "Total tokens observed by direction.",
		}, []string{"direction", "provider", "upstream"}),
		CostCentsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigilbridge_cost_cents_total",
			Help: "Total estimated cost in cents.",
		}, []string{"provider", "upstream"}),
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sigilbridge_errors_total",
			Help: "Total classified errors.",
		}, []string{"type", "provider", "upstream"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sigilbridge_request_duration_seconds",
			Help:    "Request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"ingress", "provider", "upstream"}),
		TimeToFirstByte: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sigilbridge_ttfb_seconds",
			Help:    "Time to first byte in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"provider", "upstream"}),
		InflightRequests: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sigilbridge_inflight_requests",
			Help: "Current in-flight requests.",
		}, []string{"provider", "upstream"}),
		UpstreamHealth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sigilbridge_upstream_health",
			Help: "Upstream health state: 0=down, 1=degraded, 2=healthy.",
		}, []string{"provider", "upstream"}),
		CircuitBreakerState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sigilbridge_circuit_breaker_state",
			Help: "Circuit breaker state: 0=closed, 1=open, 2=half_open.",
		}, []string{"provider", "upstream"}),
		BudgetUsedCents: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sigilbridge_budget_used_cents",
			Help: "Budget used in cents.",
		}, []string{"key_id", "period"}),
		BudgetLimitCents: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sigilbridge_budget_limit_cents",
			Help: "Budget limit in cents.",
		}, []string{"key_id", "period"}),
		SessionExpiry: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "sigilbridge_session_expiry_seconds",
			Help: "Seconds until subscription credential expiry.",
		}, []string{"session_id"}),
	}

	for _, collector := range []prometheus.Collector{
		m.RequestsTotal,
		m.TokensTotal,
		m.CostCentsTotal,
		m.ErrorsTotal,
		m.RequestDuration,
		m.TimeToFirstByte,
		m.InflightRequests,
		m.UpstreamHealth,
		m.CircuitBreakerState,
		m.BudgetUsedCents,
		m.BudgetLimitCents,
		m.SessionExpiry,
	} {
		if err := reg.Register(collector); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func Handler(reg *prometheus.Registry) http.Handler {
	if reg == nil {
		reg = Registry()
	}
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
