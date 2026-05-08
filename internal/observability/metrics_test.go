package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsHandlerReturnsPrometheusText(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics, err := NewMetrics(reg)
	if err != nil {
		t.Fatalf("NewMetrics() error = %v", err)
	}
	metrics.RequestsTotal.WithLabelValues("openai", "sonnet", "mock", "mock-a", "ok").Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler(reg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "sigilbridge_requests_total") {
		t.Fatalf("metrics body did not include request counter:\n%s", body)
	}
}
