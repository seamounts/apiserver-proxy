package server

import (
	"context"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
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
	storageRegistry map[schema.GroupVersionResource]Storage
	hookRegistry    *HookRegistry
}

type APIGroupInfo struct {
	GroupVersion                 schema.GroupVersion
	VersionedResourcesStorageMap map[string]map[string]Storage
	Scheme                       *runtime.Scheme
	NegotiatedSerializer         runtime.NegotiatedSerializer
	ParameterCodec               runtime.ParameterCodec
}

type ValidateObjectFunc func(ctx context.Context, obj runtime.Object) error
type ValidateObjectUpdateFunc func(ctx context.Context, newObj, oldObj runtime.Object) error

type UpdatedObjectInfo interface {
	UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error)
}

type Storage interface {
	New() runtime.Object
	Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error)
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
	List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error)
	Update(ctx context.Context, name string, objInfo UpdatedObjectInfo, createValidation ValidateObjectFunc, updateValidation ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error)
	Delete(ctx context.Context, name string, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error)
	DeleteCollection(ctx context.Context, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metav1.ListOptions) (runtime.Object, error)
	Watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error)
}

type RESTStorageOptions struct {
	ProxyEnabled bool
	ProxyVerbs   []string
	CustomVerbs  map[string]Storage
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
