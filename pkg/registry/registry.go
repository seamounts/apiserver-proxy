package registry

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

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

type StandardStorage interface {
	Storage
	NamespaceScoped() bool
	GetSingularName() string
}

type RESTStorage struct {
	Storage
	SingularName      string
	IsNamespaceScoped bool
}

func NewRESTStorage(storage Storage, singularName string, namespaceScoped bool) *RESTStorage {
	return &RESTStorage{
		Storage:           storage,
		SingularName:      singularName,
		IsNamespaceScoped: namespaceScoped,
	}
}

func (s *RESTStorage) NamespaceScoped() bool {
	return s.IsNamespaceScoped
}

func (s *RESTStorage) GetSingularName() string {
	return s.SingularName
}

type APIGroupInfo struct {
	GroupVersion                 schema.GroupVersion
	VersionedResourcesStorageMap map[string]map[string]Storage
	Scheme                       *runtime.Scheme
	NegotiatedSerializer         runtime.NegotiatedSerializer
	ParameterCodec               runtime.ParameterCodec
}

func NewDefaultAPIGroupInfo(group string, scheme *runtime.Scheme, parameterCodec runtime.ParameterCodec, serializer runtime.NegotiatedSerializer) APIGroupInfo {
	return APIGroupInfo{
		GroupVersion: schema.GroupVersion{Group: group, Version: "v1"},
		VersionedResourcesStorageMap: make(map[string]map[string]Storage),
		Scheme:                       scheme,
		ParameterCodec:               parameterCodec,
		NegotiatedSerializer:         serializer,
	}
}

type RESTStorageProvider interface {
	GroupName() string
	NewRESTStorage(apiResourceConfigSource APIResourceConfigSource, scheme *runtime.Scheme) (*RESTStorageBuilder, error)
}

type RESTStorageBuilder struct {
	GroupVersion           schema.GroupVersion
	VersionedStorage       map[string]map[string]Storage
	ResourceSingularName   map[string]string
	ResourceNamespaceScoped map[string]bool
}

func NewRESTStorageBuilder(groupVersion schema.GroupVersion) *RESTStorageBuilder {
	return &RESTStorageBuilder{
		GroupVersion:           groupVersion,
		VersionedStorage:       make(map[string]map[string]Storage),
		ResourceSingularName:   make(map[string]string),
		ResourceNamespaceScoped: make(map[string]bool),
	}
}

func (b *RESTStorageBuilder) AddStorage(version, resource string, storage Storage, singularName string, namespaceScoped bool) {
	if b.VersionedStorage[version] == nil {
		b.VersionedStorage[version] = make(map[string]Storage)
	}
	b.VersionedStorage[version][resource] = storage
	b.ResourceSingularName[resource] = singularName
	b.ResourceNamespaceScoped[resource] = namespaceScoped
}

type APIResourceConfigSource interface {
	ResourceEnabled(gvr schema.GroupVersionResource) bool
	AnyResourcesForGroupEnabled(group string) bool
}

type ResourceConfig struct {
	GroupVersionConfigs map[schema.GroupVersion]bool
	ResourceConfigs     map[schema.GroupVersionResource]bool
}

func NewResourceConfig() *ResourceConfig {
	return &ResourceConfig{
		GroupVersionConfigs: make(map[schema.GroupVersion]bool),
		ResourceConfigs:     make(map[schema.GroupVersionResource]bool),
	}
}

func (c *ResourceConfig) EnableVersions(versions ...schema.GroupVersion) {
	for _, v := range versions {
		c.GroupVersionConfigs[v] = true
	}
}

func (c *ResourceConfig) EnableResources(resources ...schema.GroupVersionResource) {
	for _, r := range resources {
		c.ResourceConfigs[r] = true
	}
}

func (c *ResourceConfig) ResourceEnabled(gvr schema.GroupVersionResource) bool {
	if enabled, exists := c.ResourceConfigs[gvr]; exists {
		return enabled
	}
	gv := gvr.GroupVersion()
	if enabled, exists := c.GroupVersionConfigs[gv]; exists {
		return enabled
	}
	return false
}

func (c *ResourceConfig) AnyResourcesForGroupEnabled(group string) bool {
	for gv := range c.GroupVersionConfigs {
		if gv.Group == group {
			return true
		}
	}
	for gvr := range c.ResourceConfigs {
		if gvr.Group == group {
			return true
		}
	}
	return false
}

type StorageFactory interface {
	NewStorage(gvr schema.GroupVersionResource) (Storage, error)
}

type DefaultStorageFactory struct {
	dbFactory func(gvr schema.GroupVersionResource) Storage
}

func NewDefaultStorageFactory(dbFactory func(gvr schema.GroupVersionResource) Storage) *DefaultStorageFactory {
	return &DefaultStorageFactory{
		dbFactory: dbFactory,
	}
}

func (f *DefaultStorageFactory) NewStorage(gvr schema.GroupVersionResource) (Storage, error) {
	if f.dbFactory == nil {
		return nil, fmt.Errorf("storage factory not configured")
	}
	return f.dbFactory(gvr), nil
}
