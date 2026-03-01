package registry

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceInfo struct {
	GVR             schema.GroupVersionResource
	SingularName    string
	NamespaceScoped bool
	ShortNames      []string
	Categories      []string
	Verbs           []string
	Storage         Storage
	ObjectType      runtime.Object
	ListObjectType  runtime.Object
}

type ResourceBuilder struct {
	info    *ResourceInfo
	storage Storage
	factory StorageFactory
}

func NewResourceBuilder(gvr schema.GroupVersionResource) *ResourceBuilder {
	return &ResourceBuilder{
		info: &ResourceInfo{
			GVR:             gvr,
			NamespaceScoped: true,
			Verbs:           []string{"get", "list", "create", "update", "delete"},
		},
	}
}

func (b *ResourceBuilder) SingularName(name string) *ResourceBuilder {
	b.info.SingularName = name
	return b
}

func (b *ResourceBuilder) NamespaceScoped(scoped bool) *ResourceBuilder {
	b.info.NamespaceScoped = scoped
	return b
}

func (b *ResourceBuilder) ShortNames(names ...string) *ResourceBuilder {
	b.info.ShortNames = append(b.info.ShortNames, names...)
	return b
}

func (b *ResourceBuilder) Categories(categories ...string) *ResourceBuilder {
	b.info.Categories = append(b.info.Categories, categories...)
	return b
}

func (b *ResourceBuilder) Verbs(verbs ...string) *ResourceBuilder {
	b.info.Verbs = verbs
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
	b.info.ObjectType = obj
	return b
}

func (b *ResourceBuilder) ListObjectType(list runtime.Object) *ResourceBuilder {
	b.info.ListObjectType = list
	return b
}

func (b *ResourceBuilder) Build() (*ResourceInfo, error) {
	if b.info.SingularName == "" {
		b.info.SingularName = b.info.GVR.Resource
	}

	if b.storage == nil && b.factory != nil {
		storage, err := b.factory.NewStorage(b.info.GVR)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage: %v", err)
		}
		b.storage = storage
	}

	if b.storage == nil {
		return nil, fmt.Errorf("storage is required for %s", b.info.GVR)
	}

	b.info.Storage = b.storage

	if b.info.ObjectType == nil {
		b.info.ObjectType = &metav1.PartialObjectMetadata{}
	}

	return b.info, nil
}

type ResourceRegistry struct {
	resources map[string]*ResourceInfo
	groups    map[string]*GroupInfo
}

type GroupInfo struct {
	GroupVersion schema.GroupVersion
	Resources    []*ResourceInfo
}

func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources: make(map[string]*ResourceInfo),
		groups:    make(map[string]*GroupInfo),
	}
}

func (r *ResourceRegistry) Register(builder *ResourceBuilder) error {
	info, err := builder.Build()
	if err != nil {
		return err
	}

	key := gvrKey(info.GVR)
	r.resources[key] = info

	gvKey := info.GVR.GroupVersion().String()
	if _, exists := r.groups[gvKey]; !exists {
		r.groups[gvKey] = &GroupInfo{
			GroupVersion: info.GVR.GroupVersion(),
			Resources:    make([]*ResourceInfo, 0),
		}
	}
	r.groups[gvKey].Resources = append(r.groups[gvKey].Resources, info)

	return nil
}

func (r *ResourceRegistry) Get(gvr schema.GroupVersionResource) (*ResourceInfo, bool) {
	info, exists := r.resources[gvrKey(gvr)]
	return info, exists
}

func (r *ResourceRegistry) GetStorage(gvr schema.GroupVersionResource) (Storage, bool) {
	info, exists := r.resources[gvrKey(gvr)]
	if !exists {
		return nil, false
	}
	return info.Storage, true
}

func (r *ResourceRegistry) ListResources() []*ResourceInfo {
	result := make([]*ResourceInfo, 0, len(r.resources))
	for _, info := range r.resources {
		result = append(result, info)
	}
	return result
}

func (r *ResourceRegistry) ListGroups() []*GroupInfo {
	result := make([]*GroupInfo, 0, len(r.groups))
	for _, group := range r.groups {
		result = append(result, group)
	}
	return result
}

func gvrKey(gvr schema.GroupVersionResource) string {
	return fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
}

type GroupRegistrar interface {
	GroupName() string
	RegisterResources(registry *ResourceRegistry, factory StorageFactory) error
}

func (r *ResourceRegistry) RegisterGroup(group GroupRegistrar, factory StorageFactory) error {
	return group.RegisterResources(r, factory)
}

func (r *ResourceRegistry) RegisterGroups(factory StorageFactory, groups ...GroupRegistrar) error {
	for _, group := range groups {
		if err := group.RegisterResources(r, factory); err != nil {
			return fmt.Errorf("failed to register group %s: %v", group.GroupName(), err)
		}
	}
	return nil
}
