// Package metrics exposes Prometheus metrics on /metrics (reachable only inside the Docker network).
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	reg             *prometheus.Registry
	Previews        *prometheus.CounterVec
	Explanations    *prometheus.CounterVec
	OCRDuration     *prometheus.HistogramVec
	ExplainDuration prometheus.Histogram
	Redactions      *prometheus.CounterVec
	Logins          *prometheus.CounterVec
}

func New() *Metrics {
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		Previews: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "briefklar_previews_total",
			Help: "Uploaded letters by input type and outcome (ok, no_text, unsupported, too_large, busy, error)"}, []string{"input", "outcome"}),
		Explanations: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "briefklar_explanations_total",
			Help: "Explanations by outcome (ok, limit, timeout, error)"}, []string{"outcome"}),
		OCRDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "briefklar_ocr_duration_seconds",
			Help: "Text extraction time", Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 20, 45}}, []string{"input"}),
		ExplainDuration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "briefklar_explain_duration_seconds",
			Help: "LLM call time", Buckets: []float64{1, 2, 5, 10, 20, 30, 60}}),
		Redactions: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "briefklar_redactions_total",
			Help: "Redacted values by kind"}, []string{"kind"}),
		Logins: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "briefklar_logins_total",
			Help: "Logins by outcome (success, failed, throttled)"}, []string{"outcome"}),
	}
	m.reg.MustRegister(m.Previews, m.Explanations, m.OCRDuration, m.ExplainDuration, m.Redactions, m.Logins,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *Metrics) Handler() http.Handler { return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}) }
