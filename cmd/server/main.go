package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/seamounts/apiserver-proxy/pkg/api"
	"github.com/seamounts/apiserver-proxy/pkg/middleware"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"github.com/seamounts/apiserver-proxy/pkg/server"
	"github.com/seamounts/apiserver-proxy/pkg/storage"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	var (
		kubeAPIServerURL = flag.String("kube-apiserver", "http://localhost:8080", "kube-apiserver URL")
		kubeConfig       = flag.String("kubeconfig", "", "path to kubeconfig file")
		securePort       = flag.Int("secure-port", 6443, "secure port")
		insecurePort     = flag.Int("insecure-port", 8080, "insecure port")
		dbDSN            = flag.String("db-dsn", "", "database DSN")
		enableAudit      = flag.Bool("enable-audit", true, "enable audit middleware")
		enableMetrics    = flag.Bool("enable-metrics", true, "enable metrics middleware")
		enableLogging    = flag.Bool("enable-logging", true, "enable logging middleware")
	)
	flag.Parse()

	cfg := server.NewConfig()
	cfg.KubeAPIServerURL = *kubeAPIServerURL
	cfg.KubeConfig = *kubeConfig
	cfg.SecurePort = *securePort
	cfg.InsecurePort = *insecurePort
	cfg.DBConfig = &server.DatabaseConfig{
		Driver: "mysql",
		DSN:    *dbDSN,
	}
	cfg.MiddlewareConfig = &server.MiddlewareConfig{
		EnableAudit:   *enableAudit,
		EnableMetrics: *enableMetrics,
		EnableLogging: *enableLogging,
	}

	srv, err := server.NewContainerServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create container server: %v\n", err)
		os.Exit(1)
	}

	if *dbDSN != "" {
		db, err := gorm.Open(mysql.Open(*dbDSN), &gorm.Config{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
			os.Exit(1)
		}

		resourceRegistry := registry.NewResourceRegistry()

		storageFactory := registry.StorageFactoryFunc(func(gvr schema.GroupVersionResource) (registry.Storage, error) {
			return &dbStorageAdapter{DBStorage: storage.NewDBStorage(db, nil, gvr)}, nil
		})

		apiManager := api.DefaultAPIManager()
		if err := apiManager.RegisterAll(resourceRegistry, storageFactory); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register API groups: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Registered API groups: %v\n", apiManager.ListGroups())

		customGVR := schema.GroupVersionResource{
			Group:    "example.com",
			Version:  "v1",
			Resource: "myresources",
		}
		if err := resourceRegistry.Register(
			registry.NewResourceBuilder(customGVR).
				SingularName("myresource").
				NamespaceScoped(true).
				ShortNames("mr").
				StorageFactory(storageFactory),
		); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register custom resource: %v\n", err)
			os.Exit(1)
		}

		for _, info := range resourceRegistry.ListResources() {
			srv.RegisterResource(info.GVR, info.Storage, nil)
			fmt.Printf("Registered resource: %s\n", info.GVR)
		}

		srv.SetResourceRegistry(resourceRegistry)
	}

	if *enableAudit {
		auditMiddleware := middleware.NewAuditMiddleware(&middleware.AuditConfig{
			Level: middleware.AuditLevelRequestResponse,
		}, nil)
		srv.AddMiddleware(auditMiddleware)
	}

	if *enableMetrics {
		metricsMiddleware := middleware.NewMetricsMiddleware(&middleware.MetricsConfig{
			Enabled:   true,
			Path:      "/metrics",
			Namespace: "container_server",
			Subsystem: "api",
		})
		srv.AddMiddleware(metricsMiddleware)
	}

	if *enableLogging {
		loggingMiddleware := middleware.NewLoggingMiddleware(&middleware.LoggingConfig{
			Enabled: true,
			Level:   "info",
			Format:  "json",
		})
		srv.AddMiddleware(loggingMiddleware)
	}

	srv.AddHook(server.PostCreateHook, func(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
		meta, ok := obj.(metav1.Object)
		if !ok {
			return fmt.Errorf("expected metav1.Object, got %T", obj)
		}
		fmt.Printf("Resource created: %s/%s\n", gvr.Resource, meta.GetName())
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("Shutting down...")
		cancel()
	}()

	fmt.Printf("Starting container server on ports %d (insecure) and %d (secure)\n", *insecurePort, *securePort)
	if err := srv.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

type dbStorageAdapter struct {
	*storage.DBStorage
}

func (a *dbStorageAdapter) Create(ctx context.Context, obj runtime.Object, createValidation registry.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	var f storage.ValidateObjectFunc
	if createValidation != nil {
		f = func(ctx context.Context, obj runtime.Object) error {
			return createValidation(ctx, obj)
		}
	}
	return a.DBStorage.Create(ctx, obj, f, options)
}

func (a *dbStorageAdapter) Update(ctx context.Context, name string, objInfo registry.UpdatedObjectInfo, createValidation registry.ValidateObjectFunc, updateValidation registry.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	var f storage.ValidateObjectFunc
	if createValidation != nil {
		f = func(ctx context.Context, obj runtime.Object) error {
			return createValidation(ctx, obj)
		}
	}
	var g storage.ValidateObjectUpdateFunc
	if updateValidation != nil {
		g = func(ctx context.Context, newObj, oldObj runtime.Object) error {
			return updateValidation(ctx, newObj, oldObj)
		}
	}
	var info storage.UpdatedObjectInfo = &updatedObjectInfoAdapter{objInfo}
	return a.DBStorage.Update(ctx, name, info, f, g, forceAllowCreate, options)
}

func (a *dbStorageAdapter) Delete(ctx context.Context, name string, deleteValidation registry.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	var f storage.ValidateObjectFunc
	if deleteValidation != nil {
		f = func(ctx context.Context, obj runtime.Object) error {
			return deleteValidation(ctx, obj)
		}
	}
	return a.DBStorage.Delete(ctx, name, f, options)
}

func (a *dbStorageAdapter) DeleteCollection(ctx context.Context, deleteValidation registry.ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metav1.ListOptions) (runtime.Object, error) {
	var f storage.ValidateObjectFunc
	if deleteValidation != nil {
		f = func(ctx context.Context, obj runtime.Object) error {
			return deleteValidation(ctx, obj)
		}
	}
	return a.DBStorage.DeleteCollection(ctx, f, options, listOptions)
}

type updatedObjectInfoAdapter struct {
	registry.UpdatedObjectInfo
}

func (a *updatedObjectInfoAdapter) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	return a.UpdatedObjectInfo.UpdatedObject(ctx, oldObj)
}
