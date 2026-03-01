package server

import (
	"context"
	"net/http"

	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type Config struct {
	EtcdServers      []string
	KubeAPIServerURL string
	KubeConfig       string
	SecurePort       int
	InsecurePort     int
	EnableProfiling  bool
	EnableMetrics    bool
	DBConfig         *DatabaseConfig
	MiddlewareConfig *MiddlewareConfig
}

type DatabaseConfig struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

type MiddlewareConfig struct {
	EnableAudit   bool
	EnableMetrics bool
	EnableLogging bool
}

type ContainerServer struct {
	config          *Config
	scheme          *runtime.Scheme
	proxyTransport  http.RoundTripper
	kubeRESTConfig  *rest.Config
	middlewareChain []Middleware
	storageRegistry map[schema.GroupVersionResource]registry.Storage
	hookRegistry    *HookRegistry
}

type APIGroupInfo struct {
	GroupVersion                 schema.GroupVersion
	VersionedResourcesStorageMap map[string]map[string]registry.Storage
	Scheme                       *runtime.Scheme
	NegotiatedSerializer         runtime.NegotiatedSerializer
	ParameterCodec               runtime.ParameterCodec
}

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

type HookFunc func(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error

type HookType string

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

type Verb string

const (
	VerbGet    Verb = "get"
	VerbList   Verb = "list"
	VerbCreate Verb = "create"
	VerbUpdate Verb = "update"
	VerbPatch  Verb = "patch"
	VerbDelete Verb = "delete"
	VerbWatch  Verb = "watch"
)

type Middleware interface {
	Name() string
	Handler(next http.Handler) http.Handler
}

type RESTStorageOptions struct {
	ProxyEnabled bool
	ProxyVerbs   []string
	CustomVerbs  map[string]registry.Storage
}
