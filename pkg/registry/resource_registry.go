package registry

import (
	"context"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceMeta struct {
	GVR             schema.GroupVersionResource
	SingularName    string
	NamespaceScoped bool
	ShortNames      []string
	Categories      []string
	Verbs           []string
	Storage         Storage
	ObjectType      runtime.Object
	ListType        runtime.Object
}

type ResourceBuilder struct {
	meta    *ResourceMeta
	storage Storage
	factory StorageFactory
}

func NewResourceBuilder(gvr schema.GroupVersionResource) *ResourceBuilder {
	return &ResourceBuilder{
		meta: &ResourceMeta{
			GVR:             gvr,
			NamespaceScoped: true,
			Verbs:           []string{"get", "list", "create", "update", "delete"},
		},
	}
}

func (b *ResourceBuilder) SingularName(name string) *ResourceBuilder {
	b.meta.SingularName = name
	return b
}

func (b *ResourceBuilder) NamespaceScoped(scoped bool) *ResourceBuilder {
	b.meta.NamespaceScoped = scoped
	return b
}

func (b *ResourceBuilder) ShortNames(names ...string) *ResourceBuilder {
	b.meta.ShortNames = append(b.meta.ShortNames, names...)
	return b
}

func (b *ResourceBuilder) Categories(categories ...string) *ResourceBuilder {
	b.meta.Categories = append(b.meta.Categories, categories...)
	return b
}

func (b *ResourceBuilder) Verbs(verbs ...string) *ResourceBuilder {
	b.meta.Verbs = verbs
	return b
}

func (b *ResourceBuilder) Storage(s Storage) *ResourceBuilder {
	b.storage = s
	return b
}

func (b *ResourceBuilder) StorageFactory(factory StorageFactory) *ResourceBuilder {
	b.factory = factory
	return b
}

func (b *ResourceBuilder) ObjectType(obj runtime.Object) *ResourceBuilder {
	b.meta.ObjectType = obj
	return b
}

func (b *ResourceBuilder) ListType(list runtime.Object) *ResourceBuilder {
	b.meta.ListType = list
	return b
}

func (b *ResourceBuilder) Build() (*ResourceMeta, error) {
	if b.meta.SingularName == "" {
		b.meta.SingularName = b.meta.GVR.Resource
	}

	if b.storage == nil && b.factory != nil {
		storage, err := b.factory.NewStorage(b.meta.GVR)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage: %v", err)
		}
		b.storage = storage
	}

	if b.storage == nil {
		return nil, fmt.Errorf("storage is required")
	}

	b.meta.Storage = b.storage

	if b.meta.ObjectType == nil {
		b.meta.ObjectType = &metav1.PartialObjectMetadata{}
	}

	return b.meta, nil
}

type ResourceRegistry struct {
	resources    map[string]*ResourceMeta
	groups       map[string]*APIResourceGroup
	storageCache map[schema.GroupVersionResource]Storage
}

type APIResourceGroup struct {
	GroupVersion schema.GroupVersion
	Resources    []*ResourceMeta
}

func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources:    make(map[string]*ResourceMeta),
		groups:       make(map[string]*APIResourceGroup),
		storageCache: make(map[schema.GroupVersionResource]Storage),
	}
}

func (r *ResourceRegistry) Register(builder *ResourceBuilder) error {
	meta, err := builder.Build()
	if err != nil {
		return err
	}

	key := gvrKey(meta.GVR)
	r.resources[key] = meta
	r.storageCache[meta.GVR] = meta.Storage

	groupKey := meta.GVR.GroupVersion().String()
	if _, exists := r.groups[groupKey]; !exists {
		r.groups[groupKey] = &APIResourceGroup{
			GroupVersion: meta.GVR.GroupVersion(),
			Resources:    make([]*ResourceMeta, 0),
		}
	}
	r.groups[groupKey].Resources = append(r.groups[groupKey].Resources, meta)

	return nil
}

func (r *ResourceRegistry) RegisterFunc(fn func(*ResourceBuilder) *ResourceBuilder) error {
	builder := fn(NewResourceBuilder(schema.GroupVersionResource{}))
	return r.Register(builder)
}

func (r *ResourceRegistry) Get(gvr schema.GroupVersionResource) (*ResourceMeta, bool) {
	meta, exists := r.resources[gvrKey(gvr)]
	return meta, exists
}

func (r *ResourceRegistry) GetStorage(gvr schema.GroupVersionResource) (Storage, bool) {
	storage, exists := r.storageCache[gvr]
	return storage, exists
}

func (r *ResourceRegistry) ListResources() []*ResourceMeta {
	result := make([]*ResourceMeta, 0, len(r.resources))
	for _, meta := range r.resources {
		result = append(result, meta)
	}
	return result
}

func (r *ResourceRegistry) ListGroups() []*APIResourceGroup {
	result := make([]*APIResourceGroup, 0, len(r.groups))
	for _, group := range r.groups {
		result = append(result, group)
	}
	return result
}

func (r *ResourceRegistry) GetGroup(gv schema.GroupVersion) (*APIResourceGroup, bool) {
	group, exists := r.groups[gv.String()]
	return group, exists
}

func gvrKey(gvr schema.GroupVersionResource) string {
	return fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
}

type ResourceRegistration struct {
	GVR             schema.GroupVersionResource
	SingularName    string
	NamespaceScoped bool
	ShortNames      []string
	Categories      []string
	Verbs           []string
	ObjectType      runtime.Object
	ListType        runtime.Object
	StorageWrapper  func(Storage) Storage
}

func (r *ResourceRegistry) RegisterBatch(factory StorageFactory, registrations ...ResourceRegistration) error {
	for _, reg := range registrations {
		builder := NewResourceBuilder(reg.GVR).
			SingularName(reg.SingularName).
			NamespaceScoped(reg.NamespaceScoped).
			ShortNames(reg.ShortNames...).
			Categories(reg.Categories...).
			Verbs(reg.Verbs...).
			StorageFactory(factory)

		if reg.ObjectType != nil {
			builder.ObjectType(reg.ObjectType)
		}
		if reg.ListType != nil {
			builder.ListType(reg.ListType)
		}

		if reg.StorageWrapper != nil {
			meta, err := builder.Build()
			if err != nil {
				return fmt.Errorf("failed to build resource %s: %v", reg.GVR.Resource, err)
			}
			meta.Storage = reg.StorageWrapper(meta.Storage)
		}

		if err := r.Register(builder); err != nil {
			return err
		}
	}
	return nil
}

type ResourceGroupConfig struct {
	Group     string
	Version   string
	Resources []ResourceRegistration
}

func (r *ResourceRegistry) RegisterFromConfig(factory StorageFactory, configs ...ResourceGroupConfig) error {
	for _, cfg := range configs {
		registrations := make([]ResourceRegistration, 0, len(cfg.Resources))
		for _, res := range cfg.Resources {
			if res.GVR.Group == "" {
				res.GVR.Group = cfg.Group
			}
			if res.GVR.Version == "" {
				res.GVR.Version = cfg.Version
			}
			registrations = append(registrations, res)
		}
		if err := r.RegisterBatch(factory, registrations...); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResourceRegistry) ToRESTStorageMap() map[string]map[string]Storage {
	result := make(map[string]map[string]Storage)
	for _, group := range r.groups {
		version := group.GroupVersion.Version
		if result[version] == nil {
			result[version] = make(map[string]Storage)
		}
		for _, res := range group.Resources {
			result[version][res.GVR.Resource] = res.Storage
		}
	}
	return result
}

type TypedStorage struct {
	Storage
	objectType runtime.Object
	listType   runtime.Object
	gvr        schema.GroupVersionResource
}

func NewTypedStorage(storage Storage, gvr schema.GroupVersionResource, objectType, listType runtime.Object) *TypedStorage {
	return &TypedStorage{
		Storage:    storage,
		objectType: objectType,
		listType:   listType,
		gvr:        gvr,
	}
}

func (s *TypedStorage) New() runtime.Object {
	if s.objectType != nil {
		return reflect.New(reflect.TypeOf(s.objectType).Elem()).Interface().(runtime.Object)
	}
	return &metav1.PartialObjectMetadata{}
}

func (s *TypedStorage) Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	if meta, ok := obj.(metav1.Object); ok && meta.GetNamespace() == "" && s.isNamespaceScoped() {
		meta.SetNamespace("default")
	}
	return s.Storage.Create(ctx, obj, createValidation, options)
}

func (s *TypedStorage) isNamespaceScoped() bool {
	return true
}

func CoreResourcesConfig() ResourceGroupConfig {
	return ResourceGroupConfig{
		Group:   "",
		Version: "v1",
		Resources: []ResourceRegistration{
			{
				GVR:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
				SingularName:    "pod",
				NamespaceScoped: true,
				ShortNames:      []string{"po"},
				Categories:      []string{"all"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
				SingularName:    "service",
				NamespaceScoped: true,
				ShortNames:      []string{"svc"},
				Categories:      []string{"all"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
				SingularName:    "configmap",
				NamespaceScoped: true,
				ShortNames:      []string{"cm"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
				SingularName:    "secret",
				NamespaceScoped: true,
				ShortNames:      []string{"sec"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
				SingularName:    "namespace",
				NamespaceScoped: false,
				ShortNames:      []string{"ns"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
				SingularName:    "node",
				NamespaceScoped: false,
				ShortNames:      []string{"no"},
			},
		},
	}
}

func AppsResourcesConfig() ResourceGroupConfig {
	return ResourceGroupConfig{
		Group:   "apps",
		Version: "v1",
		Resources: []ResourceRegistration{
			{
				GVR:             schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				SingularName:    "deployment",
				NamespaceScoped: true,
				ShortNames:      []string{"deploy"},
				Categories:      []string{"all"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
				SingularName:    "replicaset",
				NamespaceScoped: true,
				ShortNames:      []string{"rs"},
				Categories:      []string{"all"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
				SingularName:    "daemonset",
				NamespaceScoped: true,
				ShortNames:      []string{"ds"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
				SingularName:    "statefulset",
				NamespaceScoped: true,
				ShortNames:      []string{"sts"},
			},
		},
	}
}

func BatchResourcesConfig() ResourceGroupConfig {
	return ResourceGroupConfig{
		Group:   "batch",
		Version: "v1",
		Resources: []ResourceRegistration{
			{
				GVR:             schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
				SingularName:    "job",
				NamespaceScoped: true,
				Categories:      []string{"all"},
			},
			{
				GVR:             schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
				SingularName:    "cronjob",
				NamespaceScoped: true,
				ShortNames:      []string{"cj"},
			},
		},
	}
}

func AllBuiltinResourcesConfigs() []ResourceGroupConfig {
	return []ResourceGroupConfig{
		CoreResourcesConfig(),
		AppsResourcesConfig(),
		BatchResourcesConfig(),
	}
}
