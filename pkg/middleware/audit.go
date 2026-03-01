package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AuditMiddleware struct {
	config  *AuditConfig
	auditor Auditor
}

type Auditor interface {
	Record(ctx context.Context, event *AuditEvent) error
}

func NewAuditMiddleware(config *AuditConfig, auditor Auditor) *AuditMiddleware {
	return &AuditMiddleware{
		config:  config,
		auditor: auditor,
	}
}

func (m *AuditMiddleware) Name() string {
	return "audit"
}

func (m *AuditMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.config.Level == AuditLevelNone {
			next.ServeHTTP(w, r)
			return
		}

		startTime := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		gvr, namespace, name, verb, _ := parseRequestPath(r.URL.Path)

		event := &AuditEvent{
			Stage:      "RequestReceived",
			RequestURI: r.URL.String(),
			Verb:       verb,
			User:       nil,
			SourceIPs:  []string{r.RemoteAddr},
			UserAgent:  r.UserAgent(),
			ObjectRef: &ObjectReference{
				Group:     gvr.Group,
				Version:   gvr.Version,
				Resource:  gvr.Resource,
				Namespace: namespace,
				Name:      name,
			},
			Timestamp:   startTime,
			Annotations: make(map[string]string),
		}

		if m.config.Level == AuditLevelRequest || m.config.Level == AuditLevelRequestResponse {
			if r.Body != nil {
				bodyBytes, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				var obj interface{}
				if err := json.Unmarshal(bodyBytes, &obj); err == nil {
					event.RequestObject = obj
				}
			}
		}

		ctx := context.WithValue(r.Context(), "audit_event", event)
		ctx = context.WithValue(ctx, "request_id", requestID)
		r = r.WithContext(ctx)

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(rw, r)

		event.Duration = time.Since(startTime)
		event.Stage = "ResponseComplete"
		event.ResponseStatus = &metav1.Status{
			Status:  metav1.StatusSuccess,
			Code:    int32(rw.statusCode),
			Message: http.StatusText(rw.statusCode),
		}

		if m.config.Level == AuditLevelRequestResponse && rw.body.Len() > 0 {
			var obj interface{}
			if err := json.Unmarshal(rw.body.Bytes(), &obj); err == nil {
				event.ResponseObject = obj
			}
		}

		if m.auditor != nil {
			m.auditor.Record(ctx, event)
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func parseRequestPath(path string) (gvr schema.GroupVersionResource, namespace string, name string, verb string, subresource string) {
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
