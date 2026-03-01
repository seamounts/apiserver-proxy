package middleware

import (
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type LoggingMiddleware struct {
	config *LoggingConfig
	logger *zap.Logger
}

func NewLoggingMiddleware(config *LoggingConfig) *LoggingMiddleware {
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

	return &LoggingMiddleware{
		config: config,
		logger: logger,
	}
}

func (m *LoggingMiddleware) Name() string {
	return "logging"
}

func (m *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = "unknown"
		}

		gvr, namespace, name, verb, _ := parseLoggingRequestPath(r.URL.Path)

		m.logger.Info("request started",
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
			zap.String("group", gvr.Group),
			zap.String("version", gvr.Version),
			zap.String("resource", gvr.Resource),
			zap.String("namespace", namespace),
			zap.String("name", name),
			zap.String("verb", verb),
		)

		rw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		duration := time.Since(startTime)

		m.logger.Info("request completed",
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.statusCode),
			zap.Duration("duration", duration),
			zap.String("group", gvr.Group),
			zap.String("version", gvr.Version),
			zap.String("resource", gvr.Resource),
			zap.String("namespace", namespace),
			zap.String("name", name),
			zap.String("verb", verb),
		)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *loggingResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

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
