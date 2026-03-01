package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AuditFilter implements audit logging using go-restful filter.
type AuditFilter struct {
	config  *AuditConfig
	auditor Auditor
}

// Auditor interface for recording audit events.
type Auditor interface {
	Record(ctx context.Context, event *AuditEvent) error
}

// NewAuditFilter creates a new AuditFilter.
func NewAuditFilter(config *AuditConfig, auditor Auditor) *AuditFilter {
	return &AuditFilter{
		config:  config,
		auditor: auditor,
	}
}

// Name returns the filter name.
func (f *AuditFilter) Name() string {
	return "audit"
}

// Filter is the go-restful filter function for audit logging.
func (f *AuditFilter) Filter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	if f.config.Level == AuditLevelNone {
		chain.ProcessFilter(req, resp)
		return
	}

	startTime := time.Now()
	requestID := req.HeaderParameter("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	gvr, namespace, name, verb, _ := parseRequestPath(req.Request.URL.Path)

	event := &AuditEvent{
		Stage:      "RequestReceived",
		RequestURI: req.Request.URL.String(),
		Verb:       verb,
		User:       nil,
		SourceIPs:  []string{req.Request.RemoteAddr},
		UserAgent:  req.HeaderParameter("User-Agent"),
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

	if f.config.Level == AuditLevelRequest || f.config.Level == AuditLevelRequestResponse {
		if req.Request.Body != nil {
			bodyBytes, _ := io.ReadAll(req.Request.Body)
			req.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			var obj interface{}
			if err := json.Unmarshal(bodyBytes, &obj); err == nil {
				event.RequestObject = obj
			}
		}
	}

	ctx := context.WithValue(req.Request.Context(), "audit_event", event)
	ctx = context.WithValue(ctx, "request_id", requestID)
	req.Request = req.Request.WithContext(ctx)

	rw := &auditResponseWriter{
		ResponseWriter: resp.ResponseWriter,
		statusCode:     200,
		body:           &bytes.Buffer{},
	}
	resp.ResponseWriter = rw

	chain.ProcessFilter(req, resp)

	event.Duration = time.Since(startTime)
	event.Stage = "ResponseComplete"
	event.ResponseStatus = &metav1.Status{
		Status:  metav1.StatusSuccess,
		Code:    int32(rw.statusCode),
		Message: "OK",
	}

	if f.config.Level == AuditLevelRequestResponse && rw.body.Len() > 0 {
		var obj interface{}
		if err := json.Unmarshal(rw.body.Bytes(), &obj); err == nil {
			event.ResponseObject = obj
		}
	}

	if f.auditor != nil {
		f.auditor.Record(ctx, event)
	}
}

// auditResponseWriter wraps http.ResponseWriter to capture response data.
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// WriteHeader captures the status code.
func (rw *auditResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body.
func (rw *auditResponseWriter) Write(data []byte) (int, error) {
	rw.body.Write(data)
	return rw.ResponseWriter.Write(data)
}

// parseRequestPath parses the request path to extract GVR, namespace, name, verb, and subresource.
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
