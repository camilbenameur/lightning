// Package metrics defines the Prometheus metrics exported by the service.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the service collectors on a dedicated registry.
type Metrics struct {
	registry *prometheus.Registry

	runs         *prometheus.CounterVec
	runDuration  *prometheus.HistogramVec
	activeRuns   prometheus.Gauge
	queueWait    prometheus.Histogram
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

// New creates the collectors, including Go runtime and process metrics.
func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "lightning_runs_total",
			Help: "Code runs by language and outcome.",
		}, []string{"language", "outcome"}),
		runDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "lightning_run_duration_seconds",
			Help:    "Sandbox execution time of completed runs.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 3, 5, 8, 12, 20, 30},
		}, []string{"language"}),
		activeRuns: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "lightning_active_sandboxes",
			Help: "Sandboxes currently executing.",
		}),
		queueWait: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "lightning_queue_wait_seconds",
			Help:    "Time runs waited for a free sandbox slot.",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 20},
		}),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "lightning_http_requests_total",
			Help: "HTTP requests by route and status code.",
		}, []string{"route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "lightning_http_request_duration_seconds",
			Help:    "HTTP request latency by route.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route"}),
	}
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.runs, m.runDuration, m.activeRuns, m.queueWait, m.httpRequests, m.httpDuration,
	)
	return m
}

// Handler serves the metrics in the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// RunFinished records the outcome of a run; duration is zero for runs that
// never executed.
func (m *Metrics) RunFinished(language, outcome string, duration time.Duration) {
	m.runs.WithLabelValues(language, outcome).Inc()
	if duration > 0 {
		m.runDuration.WithLabelValues(language).Observe(duration.Seconds())
	}
}

// SandboxStarted records that a sandbox began executing.
func (m *Metrics) SandboxStarted() { m.activeRuns.Inc() }

// SandboxFinished records that a sandbox stopped executing.
func (m *Metrics) SandboxFinished() { m.activeRuns.Dec() }

// QueueWaited records how long a run waited for a sandbox slot.
func (m *Metrics) QueueWaited(d time.Duration) { m.queueWait.Observe(d.Seconds()) }

// HTTPRequest records a served request. route must have bounded cardinality.
func (m *Metrics) HTTPRequest(route string, status int, d time.Duration) {
	m.httpRequests.WithLabelValues(route, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(route).Observe(d.Seconds())
}
