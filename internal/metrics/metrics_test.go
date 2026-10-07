package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerExposesCounters(t *testing.T) {
	m := New()
	m.Logins.WithLabelValues("success").Inc()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(rec.Body.String(), `briefklar_logins_total{outcome="success"} 1`) {
		t.Fatalf("metrics: %s", rec.Body)
	}
}
