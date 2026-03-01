// Package middleware provides HTTP middleware implementations for the API server.
// It includes audit logging, metrics collection, and request logging middleware.
package middleware

import (
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Middleware is the interface for HTTP middleware.
type Middleware interface {
	// Name returns the middleware name for identification.
	Name() string
	// Handler wraps the next HTTP handler with middleware functionality.
	Handler(next http.Handler) http.Handler
}

// MiddlewareChain manages a chain of middleware.
// Middleware is executed in the order they were added.
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new empty middleware chain.
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
	}
}

// Add appends a middleware to the chain.
func (c *MiddlewareChain) Add(m Middleware) {
	c.middlewares = append(c.middlewares, m)
}

// Handler builds the final handler by chaining all middleware.
// Middleware is executed in reverse order so that the first added
// middleware is the outermost wrapper.
func (c *MiddlewareChain) Handler(final http.Handler) http.Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		final = c.middlewares[i].Handler(final)
	}
	return final
}

// RequestContext contains request-scoped information passed through middleware.
type RequestContext struct {
	// RequestID is a unique identifier for the request
	RequestID string
	// UserAgent is the client's user agent string
	UserAgent string
	// RemoteAddr is the client's IP address
	RemoteAddr string
	// Method is the HTTP method
	Method string
	// Path is the request path
	Path string
	// GVR is the GroupVersionResource being accessed
	GVR schema.GroupVersionResource
	// Namespace is the resource namespace (if applicable)
	Namespace string
	// Name is the resource name (if applicable)
	Name string
	// Verb is the API verb (get, list, create, update, delete)
	Verb string
	// SubResource is the subresource being accessed (if applicable)
	SubResource string
	// StartTime is when the request started processing
	StartTime time.Time
	// UserInfo contains authenticated user information
	UserInfo interface{}
	// AuditAnnotations are additional audit annotations
	AuditAnnotations map[string]string
}

// AuditEvent represents an audit event for logging.
type AuditEvent struct {
	// Stage is the audit stage (RequestReceived, ResponseStarted, ResponseComplete)
	Stage string
	// RequestURI is the full request URI
	RequestURI string
	// Verb is the API verb
	Verb string
	// User is the authenticated user information
	User interface{}
	// SourceIPs are the source IP addresses
	SourceIPs []string
	// UserAgent is the client's user agent
	UserAgent string
	// ObjectRef is a reference to the object being accessed
	ObjectRef *ObjectReference
	// ResponseStatus is the HTTP response status
	ResponseStatus *metav1.Status
	// RequestObject is the request body object
	RequestObject interface{}
	// ResponseObject is the response body object
	ResponseObject interface{}
	// Timestamp is when the event occurred
	Timestamp time.Time
	// Duration is how long the request took
	Duration time.Duration
	// Annotations are additional audit annotations
	Annotations map[string]string
}

// ObjectReference is a reference to a Kubernetes object.
type ObjectReference struct {
	Group      string
	Version    string
	Resource   string
	Namespace  string
	Name       string
	UID        string
	APIGroup   string
	APIVersion string
}

// AuditLevel defines the level of audit logging.
type AuditLevel string

// Audit level constants.
const (
	// AuditLevelNone disables audit logging
	AuditLevelNone AuditLevel = "None"
	// AuditLevelMetadata logs only request metadata
	AuditLevelMetadata AuditLevel = "Metadata"
	// AuditLevelMetadata logs request metadata and request body
	AuditLevelRequest AuditLevel = "Request"
	// AuditLevelRequestResponse logs request metadata, request body, and response body
	AuditLevelRequestResponse AuditLevel = "RequestResponse"
)

// AuditConfig holds configuration for audit middleware.
type AuditConfig struct {
	// Level is the audit logging level
	Level AuditLevel
	// LogPath is the path to the audit log file
	LogPath string
	// MaxAge is the maximum age of log files in days
	MaxAge int
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	// MaxSize is the maximum size of log files in megabytes
	MaxSize int
}

// MetricsConfig holds configuration for metrics middleware.
type MetricsConfig struct {
	// Enabled indicates if metrics collection is enabled
	Enabled bool
	// Path is the HTTP path for the metrics endpoint
	Path string
	// Namespace is the Prometheus metrics namespace
	Namespace string
	// Subsystem is the Prometheus metrics subsystem
	Subsystem string
}

// LoggingConfig holds configuration for logging middleware.
type LoggingConfig struct {
	// Enabled indicates if request logging is enabled
	Enabled bool
	// Level is the log level (debug, info, warn, error)
	Level string
	// Format is the log format (json, text)
	Format string
	// OutputPath is the path for log output
	OutputPath string
}
