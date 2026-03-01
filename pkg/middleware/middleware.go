package middleware

import (
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Middleware interface {
	Name() string
	Handler(next http.Handler) http.Handler
}

type MiddlewareChain struct {
	middlewares []Middleware
}

func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
	}
}

func (c *MiddlewareChain) Add(m Middleware) {
	c.middlewares = append(c.middlewares, m)
}

func (c *MiddlewareChain) Handler(final http.Handler) http.Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		final = c.middlewares[i].Handler(final)
	}
	return final
}

type RequestContext struct {
	RequestID        string
	UserAgent        string
	RemoteAddr       string
	Method           string
	Path             string
	GVR              schema.GroupVersionResource
	Namespace        string
	Name             string
	Verb             string
	SubResource      string
	StartTime        time.Time
	UserInfo         interface{}
	AuditAnnotations map[string]string
}

type AuditEvent struct {
	Stage          string
	RequestURI     string
	Verb           string
	User           interface{}
	SourceIPs      []string
	UserAgent      string
	ObjectRef      *ObjectReference
	ResponseStatus *metav1.Status
	RequestObject  interface{}
	ResponseObject interface{}
	Timestamp      time.Time
	Duration       time.Duration
	Annotations    map[string]string
}

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

type AuditLevel string

const (
	AuditLevelNone            AuditLevel = "None"
	AuditLevelMetadata        AuditLevel = "Metadata"
	AuditLevelRequest         AuditLevel = "Request"
	AuditLevelRequestResponse AuditLevel = "RequestResponse"
)

type AuditConfig struct {
	Level      AuditLevel
	LogPath    string
	MaxAge     int
	MaxBackups int
	MaxSize    int
}

type MetricsConfig struct {
	Enabled   bool
	Path      string
	Namespace string
	Subsystem string
}

type LoggingConfig struct {
	Enabled    bool
	Level      string
	Format     string
	OutputPath string
}
