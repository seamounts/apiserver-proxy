package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// LoggingFilter implements request logging using go-restful filter.
type LoggingFilter struct {
	config *LoggingConfig
	logger *zap.Logger
}

// NewLoggingFilter creates a new LoggingFilter.
func NewLoggingFilter(config *LoggingConfig) *LoggingFilter {
	var logger *zap.Logger
	var err error

	if config.Enabled {
		if config.Format == "json" {
			logger, err = zap.NewProduction()
		} else {
			logger, err = zap.NewDevelopment()
		}
		if err != nil {
			logger = zap.NewNop()
		}
	} else {
		logger = zap.NewNop()
	}

	return &LoggingFilter{
		config: config,
		logger: logger,
	}
}

// Name returns the filter name.
func (f *LoggingFilter) Name() string {
	return "logging"
}

// Filter is the go-restful filter function for request logging.
func (f *LoggingFilter) Filter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	startTime := time.Now()

	requestID := req.HeaderParameter("X-Request-ID")
	if requestID == "" {
		requestID = "unknown"
	}

	gvr, namespace, name, verb, _ := parseLoggingRequestPath(req.Request.URL.Path)

	f.logger.Info("request started",
		zap.String("request_id", requestID),
		zap.String("method", req.Request.Method),
		zap.String("path", req.Request.URL.Path),
		zap.String("remote_addr", req.Request.RemoteAddr),
		zap.String("user_agent", req.HeaderParameter("User-Agent")),
		zap.String("group", gvr.Group),
		zap.String("version", gvr.Version),
		zap.String("resource", gvr.Resource),
		zap.String("namespace", namespace),
		zap.String("name", name),
		zap.String("verb", verb),
	)

	rw := &loggingResponseWriter{
		ResponseWriter: resp.ResponseWriter,
		statusCode:     200,
	}
	resp.ResponseWriter = rw

	chain.ProcessFilter(req, resp)

	duration := time.Since(startTime)

	f.logger.Info("request completed",
		zap.String("request_id", requestID),
		zap.String("method", req.Request.Method),
		zap.String("path", req.Request.URL.Path),
		zap.Int("status", rw.statusCode),
		zap.Duration("duration", duration),
		zap.String("group", gvr.Group),
		zap.String("version", gvr.Version),
		zap.String("resource", gvr.Resource),
		zap.String("namespace", namespace),
		zap.String("name", name),
		zap.String("verb", verb),
	)
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (rw *loggingResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// parseLoggingRequestPath parses the request path to extract GVR, namespace, name, verb, and subresource.
func parseLoggingRequestPath(path string) (gvr schema.GroupVersionResource, namespace string, name string, verb string, subresource string) {
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
