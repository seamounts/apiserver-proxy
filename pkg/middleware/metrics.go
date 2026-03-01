package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type MetricsMiddleware struct {
	config           *MetricsConfig
	requestCounter   *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestSize      *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	inFlightRequests *prometheus.GaugeVec
}

func NewMetricsMiddleware(config *MetricsConfig) *MetricsMiddleware {
	if config.Namespace == "" {
		config.Namespace = "container_server"
	}
	if config.Subsystem == "" {
		config.Subsystem = "api"
	}
	if config.Path == "" {
		config.Path = "/metrics"
	}

	m := &MetricsMiddleware{
		config: config,
		requestCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "requests_total",
				Help:      "Total number of API requests",
			},
			[]string{"method", "path", "status", "group", "version", "resource", "verb"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_duration_seconds",
				Help:      "Duration of API requests in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "group", "version", "resource", "verb"},
		),
		requestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_size_bytes",
				Help:      "Size of API requests in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path", "group", "version", "resource", "verb"},
		),
		responseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "response_size_bytes",
				Help:      "Size of API responses in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"method", "path", "group", "version", "resource", "verb"},
		),
		inFlightRequests: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "in_flight_requests",
				Help:      "Number of in-flight API requests",
			},
			[]string{"method", "path", "group", "version", "resource", "verb"},
		),
	}

	return m
}

func (m *MetricsMiddleware) Name() string {
	return "metrics"
}

func (m *MetricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		startTime := time.Now()
		gvr, _, _, verb, _ := parseMetricsRequestPath(r.URL.Path)

		labels := prometheus.Labels{
			"method":   r.Method,
			"path":     r.URL.Path,
			"group":    gvr.Group,
			"version":  gvr.Version,
			"resource": gvr.Resource,
			"verb":     verb,
		}

		m.inFlightRequests.With(labels).Inc()
		defer m.inFlightRequests.With(labels).Dec()

		rw := &metricsResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			size:           0,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(startTime).Seconds()

		m.requestCounter.With(prometheus.Labels{
			"method":   r.Method,
			"path":     r.URL.Path,
			"status":   strconv.Itoa(rw.statusCode),
			"group":    gvr.Group,
			"version":  gvr.Version,
			"resource": gvr.Resource,
			"verb":     verb,
		}).Inc()

		m.requestDuration.With(labels).Observe(duration)
		m.requestSize.With(labels).Observe(float64(r.ContentLength))
		m.responseSize.With(labels).Observe(float64(rw.size))
	})
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *metricsResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	rw.size += len(b)
	return rw.ResponseWriter.Write(b)
}

func parseMetricsRequestPath(path string) (gvr schema.GroupVersionResource, namespace string, name string, verb string, subresource string) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 2 && parts[0] == "api" {
		gvr.Version = parts[1]
		if len(parts) >= 3 {
			gvr.Resource = parts[2]
		}
		if len(parts) >= 4 {
			name = parts[3]
		}
		if len(parts) >= 5 {
			subresource = parts[4]
		}
	} else if len(parts) >= 3 && parts[0] == "apis" {
		gvr.Group = parts[1]
		gvr.Version = parts[2]
		if len(parts) >= 4 {
			gvr.Resource = parts[3]
		}
		if len(parts) >= 5 {
			name = parts[4]
		}
		if len(parts) >= 6 {
			subresource = parts[5]
		}
	}

	switch {
	case name == "" && subresource == "":
		verb = "list"
	case name != "" && subresource == "":
		verb = "get"
	default:
		verb = "get"
	}

	return gvr, namespace, name, verb, subresource
}
