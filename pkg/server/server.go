package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

func NewConfig() *Config {
	return &Config{
		SecurePort:      6443,
		InsecurePort:    8080,
		EnableProfiling: true,
		EnableMetrics:   true,
		DBConfig: &DatabaseConfig{
			Driver:          "mysql",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: 3600,
		},
		MiddlewareConfig: &MiddlewareConfig{
			EnableAudit:   true,
			EnableMetrics: true,
			EnableLogging: true,
		},
	}
}

func NewContainerServer(cfg *Config) (*ContainerServer, error) {
	scheme := runtime.NewScheme()

	kubeRESTConfig, err := buildKubeRESTConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build kube rest config: %v", err)
	}

	proxyTransport, err := buildProxyTransport(kubeRESTConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build proxy transport: %v", err)
	}

	s := &ContainerServer{
		config:          cfg,
		scheme:          scheme,
		kubeRESTConfig:  kubeRESTConfig,
		proxyTransport:  proxyTransport,
		storageRegistry: make(map[schema.GroupVersionResource]registry.Storage),
		hookRegistry:    NewHookRegistry(),
		middlewareChain: make([]Middleware, 0),
	}

	return s, nil
}

func buildKubeRESTConfig(cfg *Config) (*rest.Config, error) {
	if cfg.KubeConfig != "" {
		return rest.InClusterConfig()
	}

	return &rest.Config{
		Host: cfg.KubeAPIServerURL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}, nil
}

func buildProxyTransport(cfg *rest.Config) (http.RoundTripper, error) {
	transportConfig, err := cfg.TransportConfig()
	if err != nil {
		return nil, err
	}

	return transport.New(transportConfig)
}

func (s *ContainerServer) InstallAPIGroups(apiGroupInfos ...*APIGroupInfo) error {
	for _, apiGroupInfo := range apiGroupInfos {
		if err := s.installAPIGroup(apiGroupInfo); err != nil {
			return fmt.Errorf("failed to install api group: %v", err)
		}
	}
	return nil
}

func (s *ContainerServer) installAPIGroup(apiGroupInfo *APIGroupInfo) error {
	for version, storageMap := range apiGroupInfo.VersionedResourcesStorageMap {
		for resource, storage := range storageMap {
			gvr := schema.GroupVersionResource{
				Group:    apiGroupInfo.GroupVersion.Group,
				Version:  version,
				Resource: resource,
			}
			s.storageRegistry[gvr] = storage
		}
	}

	return nil
}

func (s *ContainerServer) RegisterResource(gvr schema.GroupVersionResource, storage registry.Storage, options *RESTStorageOptions) error {
	s.storageRegistry[gvr] = storage

	if options != nil && len(options.CustomVerbs) > 0 {
		for verb, customStorage := range options.CustomVerbs {
			customGVR := gvr
			customGVR.Resource = gvr.Resource + "/" + verb
			s.storageRegistry[customGVR] = customStorage
		}
	}

	return nil
}

func (s *ContainerServer) AddMiddleware(middleware Middleware) {
	s.middlewareChain = append(s.middlewareChain, middleware)
}

func (s *ContainerServer) AddHook(hookType HookType, hook HookFunc) {
	switch hookType {
	case PreCreateHook:
		s.hookRegistry.preCreateHooks = append(s.hookRegistry.preCreateHooks, hook)
	case PostCreateHook:
		s.hookRegistry.postCreateHooks = append(s.hookRegistry.postCreateHooks, hook)
	case PreUpdateHook:
		s.hookRegistry.preUpdateHooks = append(s.hookRegistry.preUpdateHooks, hook)
	case PostUpdateHook:
		s.hookRegistry.postUpdateHooks = append(s.hookRegistry.postUpdateHooks, hook)
	case PreDeleteHook:
		s.hookRegistry.preDeleteHooks = append(s.hookRegistry.preDeleteHooks, hook)
	case PostDeleteHook:
		s.hookRegistry.postDeleteHooks = append(s.hookRegistry.postDeleteHooks, hook)
	case PreGetHook:
		s.hookRegistry.preGetHooks = append(s.hookRegistry.preGetHooks, hook)
	case PostGetHook:
		s.hookRegistry.postGetHooks = append(s.hookRegistry.postGetHooks, hook)
	case PreListHook:
		s.hookRegistry.preListHooks = append(s.hookRegistry.preListHooks, hook)
	case PostListHook:
		s.hookRegistry.postListHooks = append(s.hookRegistry.postListHooks, hook)
	}
}

func (s *ContainerServer) Run(ctx context.Context) error {
	restContainer := s.setupRoutes()

	errChan := make(chan error, 1)

	go func() {
		server := &http.Server{
			Addr:         fmt.Sprintf(":%d", s.config.InsecurePort),
			Handler:      restContainer,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
		errChan <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

func (s *ContainerServer) setupRoutes() *restful.Container {
	restContainer := restful.NewContainer()
	restContainer.Router(restful.CurlyRouter{})

	ws := new(restful.WebService)
	ws.Path("/apis")
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)

	ws.Route(ws.GET("/{group}/{version}/{resource}").To(s.handleList))
	ws.Route(ws.POST("/{group}/{version}/{resource}").To(s.handleCreate))
	ws.Route(ws.GET("/{group}/{version}/{resource}/{name}").To(s.handleGet))
	ws.Route(ws.PUT("/{group}/{version}/{resource}/{name}").To(s.handleUpdate))
	ws.Route(ws.PATCH("/{group}/{version}/{resource}/{name}").To(s.handlePatch))
	ws.Route(ws.DELETE("/{group}/{version}/{resource}/{name}").To(s.handleDelete))
	ws.Route(ws.GET("/{group}/{version}/{resource}/{name}/{subresource}").To(s.handleSubresource))
	ws.Route(ws.POST("/{group}/{version}/{resource}/{name}/{subresource}").To(s.handleSubresource))

	restContainer.Add(ws)

	wsCore := new(restful.WebService)
	wsCore.Path("/api/v1")
	wsCore.Consumes(restful.MIME_JSON)
	wsCore.Produces(restful.MIME_JSON)

	wsCore.Route(wsCore.GET("/{resource}").To(s.handleCoreList))
	wsCore.Route(wsCore.POST("/{resource}").To(s.handleCoreCreate))
	wsCore.Route(wsCore.GET("/{resource}/{name}").To(s.handleCoreGet))
	wsCore.Route(wsCore.PUT("/{resource}/{name}").To(s.handleCoreUpdate))
	wsCore.Route(wsCore.PATCH("/{resource}/{name}").To(s.handleCorePatch))
	wsCore.Route(wsCore.DELETE("/{resource}/{name}").To(s.handleCoreDelete))

	restContainer.Add(wsCore)

	return restContainer
}

func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		preCreateHooks:  make([]HookFunc, 0),
		postCreateHooks: make([]HookFunc, 0),
		preUpdateHooks:  make([]HookFunc, 0),
		postUpdateHooks: make([]HookFunc, 0),
		preDeleteHooks:  make([]HookFunc, 0),
		postDeleteHooks: make([]HookFunc, 0),
		preGetHooks:     make([]HookFunc, 0),
		postGetHooks:    make([]HookFunc, 0),
		preListHooks:    make([]HookFunc, 0),
		postListHooks:   make([]HookFunc, 0),
	}
}

func (h *HookRegistry) ExecutePreCreateHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.preCreateHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePostCreateHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.postCreateHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePreUpdateHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.preUpdateHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePostUpdateHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.postUpdateHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePreDeleteHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.preDeleteHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePostDeleteHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.postDeleteHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePreGetHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.preGetHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePostGetHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.postGetHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePreListHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.preListHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}

func (h *HookRegistry) ExecutePostListHooks(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
	for _, hook := range h.postListHooks {
		if err := hook(ctx, gvr, obj); err != nil {
			return err
		}
	}
	return nil
}
