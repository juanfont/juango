package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts total HTTP requests.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks HTTP request duration.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// HTTPRequestsInFlight tracks the number of in-flight requests.
	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
)

// metricsResponseWriter wraps http.ResponseWriter to capture status code.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Metrics returns a middleware that collects Prometheus metrics.
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HTTPRequestsInFlight.Inc()
			defer HTTPRequestsInFlight.Dec()

			start := time.Now()
			wrapped := newMetricsResponseWriter(w)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Seconds()

			HTTPRequestsTotal.WithLabelValues(
				r.Method,
				r.URL.Path,
				strconv.Itoa(wrapped.statusCode),
			).Inc()

			HTTPRequestDuration.WithLabelValues(
				r.Method,
				r.URL.Path,
			).Observe(duration)
		})
	}
}

// MetricsSimple returns a simpler metrics middleware that only counts requests.
// This is useful when you don't need detailed path-level metrics.
func MetricsSimple() func(http.Handler) http.Handler {
	counter := promauto.NewCounter(prometheus.CounterOpts{
		Name: "http_requests_total_simple",
		Help: "Total number of HTTP requests (simple counter)",
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			counter.Inc()
			next.ServeHTTP(w, r)
		})
	}
}
