// Package server provides the core container server implementation.
// It handles HTTP routing, middleware, hooks, and resource registration.
package server

import (
	"context"
	"net/http"

	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

// Config holds the server configuration.
type Config struct {
	// EtcdServers is the list of etcd servers for storage
	EtcdServers []string
	// KubeAPIServerURL is the URL of the Kubernetes API server for proxying
	KubeAPIServerURL string
	// KubeConfig is the path to kubeconfig file
	KubeConfig string
	// SecurePort is the HTTPS port
	SecurePort int
	// InsecurePort is the HTTP port
	InsecurePort int
	// EnableProfiling enables pprof endpoints
	EnableProfiling bool
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
	// DBConfig is the database configuration
	DBConfig *DatabaseConfig
	// MiddlewareConfig is the middleware configuration
	MiddlewareConfig *MiddlewareConfig
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	// Driver is the database driver name (e.g., "mysql", "postgres")
	Driver string
	// DSN is the data source name/connection string
	DSN string
	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int
	// ConnMaxLifetime is the maximum lifetime of connections in seconds
	ConnMaxLifetime int
}

// MiddlewareConfig holds middleware configuration.
type MiddlewareConfig struct {
	// EnableAudit enables audit logging
	EnableAudit bool
	// EnableMetrics enables Prometheus metrics collection
	EnableMetrics bool
	// EnableLogging enables request logging
	EnableLogging bool
}

// ContainerServer is the main server that handles API requests.
// It manages resource storage, middleware chain, and hooks.
type ContainerServer struct {
	config          *Config
	scheme          *runtime.Scheme
	proxyTransport  http.RoundTripper
	kubeRESTConfig  *rest.Config
	middlewareChain []Middleware
	storageRegistry map[schema.GroupVersionResource]registry.Storage
	hookRegistry    *HookRegistry
}

// APIGroupInfo contains information about an API group.
type APIGroupInfo struct {
	// GroupVersion is the group-version identifier
	GroupVersion schema.GroupVersion
	// VersionedResourcesStorageMap maps version to resource name to storage
	VersionedResourcesStorageMap map[string]map[string]registry.Storage
	// Scheme is the runtime scheme for this API group
	Scheme *runtime.Scheme
	// NegotiatedSerializer handles encoding/decoding
	NegotiatedSerializer runtime.NegotiatedSerializer
	// ParameterCodec handles parameter conversion
	ParameterCodec runtime.ParameterCodec
}

// HookRegistry manages lifecycle hooks for resources.
type HookRegistry struct {
	preCreateHooks  []HookFunc
	postCreateHooks []HookFunc
	preUpdateHooks  []HookFunc
	postUpdateHooks []HookFunc
	preDeleteHooks  []HookFunc
	postDeleteHooks []HookFunc
	preGetHooks     []HookFunc
	postGetHooks    []HookFunc
	preListHooks    []HookFunc
	postListHooks   []HookFunc
}

// HookFunc is a function that is called during resource lifecycle events.
type HookFunc func(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error

// HookType represents the type of hook.
type HookType string

// Hook type constants.
const (
	PreCreateHook  HookType = "preCreate"
	PostCreateHook HookType = "postCreate"
	PreUpdateHook  HookType = "preUpdate"
	PostUpdateHook HookType = "postUpdate"
	PreDeleteHook  HookType = "preDelete"
	PostDeleteHook HookType = "postDelete"
	PreGetHook     HookType = "preGet"
	PostGetHook    HookType = "postGet"
	PreListHook    HookType = "preList"
	PostListHook   HookType = "postList"
)

// Verb represents an HTTP verb/operation.
type Verb string

// Verb constants.
const (
	VerbGet    Verb = "get"
	VerbList   Verb = "list"
	VerbCreate Verb = "create"
	VerbUpdate Verb = "update"
	VerbPatch  Verb = "patch"
	VerbDelete Verb = "delete"
	VerbWatch  Verb = "watch"
)

// Middleware is the interface for HTTP middleware.
type Middleware interface {
	// Name returns the middleware name
	Name() string
	// Handler wraps the next handler
	Handler(next http.Handler) http.Handler
}

// RESTStorageOptions contains options for REST storage registration.
type RESTStorageOptions struct {
	// ProxyEnabled indicates if proxying to Kubernetes API server is enabled
	ProxyEnabled bool
	// ProxyVerbs lists verbs that should be proxied
	ProxyVerbs []string
	// CustomVerbs maps custom verbs to their storage implementations
	CustomVerbs map[string]registry.Storage
}
