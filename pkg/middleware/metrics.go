package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MetricsFilter implements metrics collection using go-restful filter.
type MetricsFilter struct {
	config           *MetricsConfig
	requestCounter   *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestSize      *prometheus.HistogramVec
	responseSize     *prometheus.HistogramVec
	inFlightRequests *prometheus.GaugeVec
}

// NewMetricsFilter creates a new MetricsFilter.
func NewMetricsFilter(config *MetricsConfig) *MetricsFilter {
	if config.Namespace == "" {
		config.Namespace = "container_server"
	}
	if config.Subsystem == "" {
		config.Subsystem = "api"
	}
	if config.Path == "" {
		config.Path = "/metrics"
	}

	f := &MetricsFilter{
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

	return f
}

// Name returns the filter name.
func (f *MetricsFilter) Name() string {
	return "metrics"
}

// Filter is the go-restful filter function for metrics collection.
func (f *MetricsFilter) Filter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if !f.config.Enabled {
		chain.ProcessFilter(req, resp)
		return
	}

	startTime := time.Now()
	gvr, _, _, verb, _ := parseMetricsRequestPath(req.Request.URL.Path)

	labels := prometheus.Labels{
		"method":   req.Request.Method,
		"path":     req.Request.URL.Path,
		"group":    gvr.Group,
		"version":  gvr.Version,
		"resource": gvr.Resource,
		"verb":     verb,
	}

	f.inFlightRequests.With(labels).Inc()
	defer f.inFlightRequests.With(labels).Dec()

	rw := &metricsResponseWriter{
		ResponseWriter: resp.ResponseWriter,
		statusCode:     200,
		size:           0,
	}
	resp.ResponseWriter = rw

	chain.ProcessFilter(req, resp)

	duration := time.Since(startTime).Seconds()

	f.requestCounter.With(prometheus.Labels{
		"method":   req.Request.Method,
		"path":     req.Request.URL.Path,
		"status":   strconv.Itoa(rw.statusCode),
		"group":    gvr.Group,
		"version":  gvr.Version,
		"resource": gvr.Resource,
		"verb":     verb,
	}).Inc()

	f.requestDuration.With(labels).Observe(duration)
	f.requestSize.With(labels).Observe(float64(req.Request.ContentLength))
	f.responseSize.With(labels).Observe(float64(rw.size))
}

// metricsResponseWriter wraps http.ResponseWriter to capture status code and size.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader captures the status code.
func (rw *metricsResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response size.
func (rw *metricsResponseWriter) Write(data []byte) (int, error) {
	rw.size += len(data)
	return rw.ResponseWriter.Write(data)
}

// parseMetricsRequestPath parses the request path to extract GVR, namespace, name, verb, and subresource.
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
