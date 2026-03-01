package registry

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceInfo contains all metadata and storage for a registered resource.
type ResourceInfo struct {
	// GVR is the GroupVersionResource identifier
	GVR schema.GroupVersionResource
	// SingularName is the singular resource name
	SingularName string
	// NamespaceScoped indicates if the resource is namespaced
	NamespaceScoped bool
	// ShortNames are short aliases for the resource
	ShortNames []string
	// Categories are groups of resources this belongs to
	Categories []string
	// Verbs are the supported operations (e.g., get, list, create, update, delete)
	Verbs []string
	// Storage is the storage implementation for this resource
	Storage Storage
	// ObjectType is the Go type for the resource object
	ObjectType runtime.Object
	// ListObjectType is the Go type for the resource list object
	ListObjectType runtime.Object
}

// ResourceBuilder provides a fluent API for building ResourceInfo.
// It allows setting resource properties in a chainable manner.
type ResourceBuilder struct {
	info    *ResourceInfo
	storage Storage
	factory StorageFactory
}

// NewResourceBuilder creates a new ResourceBuilder with the given GVR.
// Default values are set: NamespaceScoped=true, Verbs=["get","list","create","update","delete"]
func NewResourceBuilder(gvr schema.GroupVersionResource) *ResourceBuilder {
	return &ResourceBuilder{
		info: &ResourceInfo{
			GVR:             gvr,
			NamespaceScoped: true,
			Verbs:           []string{"get", "list", "create", "update", "delete"},
		},
	}
}

// SingularName sets the singular name for the resource.
func (b *ResourceBuilder) SingularName(name string) *ResourceBuilder {
	b.info.SingularName = name
	return b
}

// NamespaceScoped sets whether the resource is namespaced.
func (b *ResourceBuilder) NamespaceScoped(scoped bool) *ResourceBuilder {
	b.info.NamespaceScoped = scoped
	return b
}

// ShortNames sets the short names for the resource.
func (b *ResourceBuilder) ShortNames(names ...string) *ResourceBuilder {
	b.info.ShortNames = append(b.info.ShortNames, names...)
	return b
}

// Categories sets the categories for the resource.
func (b *ResourceBuilder) Categories(categories ...string) *ResourceBuilder {
	b.info.Categories = append(b.info.Categories, categories...)
	return b
}

// Verbs sets the supported verbs for the resource.
func (b *ResourceBuilder) Verbs(verbs ...string) *ResourceBuilder {
	b.info.Verbs = verbs
	return b
}

// Storage sets the storage implementation directly.
func (b *ResourceBuilder) Storage(s Storage) *ResourceBuilder {
	b.storage = s
	return b
}

// StorageFactory sets the factory for creating storage.
func (b *ResourceBuilder) StorageFactory(factory StorageFactory) *ResourceBuilder {
	b.factory = factory
	return b
}

// ObjectType sets the Go type for the resource object.
func (b *ResourceBuilder) ObjectType(obj runtime.Object) *ResourceBuilder {
	b.info.ObjectType = obj
	return b
}

// ListObjectType sets the Go type for the resource list object.
func (b *ResourceBuilder) ListObjectType(list runtime.Object) *ResourceBuilder {
	b.info.ListObjectType = list
	return b
}

// Build creates the ResourceInfo from the builder configuration.
// It validates that storage is available and sets defaults for missing fields.
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

// ResourceRegistry manages all registered resources.
// It provides lookup and listing operations for resources and groups.
type ResourceRegistry struct {
	resources map[string]*ResourceInfo
	groups    map[string]*GroupInfo
}

// GroupInfo contains information about an API group.
type GroupInfo struct {
	// GroupVersion is the group-version identifier
	GroupVersion schema.GroupVersion
	// Resources is the list of resources in this group
	Resources []*ResourceInfo
}

// NewResourceRegistry creates a new empty ResourceRegistry.
func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources: make(map[string]*ResourceInfo),
		groups:    make(map[string]*GroupInfo),
	}
}

// Register adds a resource to the registry using a ResourceBuilder.
// The resource is added to both the resources map and the appropriate group.
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

// Get retrieves a resource by its GVR.
func (r *ResourceRegistry) Get(gvr schema.GroupVersionResource) (*ResourceInfo, bool) {
	info, exists := r.resources[gvrKey(gvr)]
	return info, exists
}

// GetStorage retrieves the storage for a resource by its GVR.
func (r *ResourceRegistry) GetStorage(gvr schema.GroupVersionResource) (Storage, bool) {
	info, exists := r.resources[gvrKey(gvr)]
	if !exists {
		return nil, false
	}
	return info.Storage, true
}

// ListResources returns all registered resources.
func (r *ResourceRegistry) ListResources() []*ResourceInfo {
	result := make([]*ResourceInfo, 0, len(r.resources))
	for _, info := range r.resources {
		result = append(result, info)
	}
	return result
}

// ListGroups returns all registered API groups.
func (r *ResourceRegistry) ListGroups() []*GroupInfo {
	result := make([]*GroupInfo, 0, len(r.groups))
	for _, group := range r.groups {
		result = append(result, group)
	}
	return result
}

// gvrKey creates a unique string key for a GVR.
func gvrKey(gvr schema.GroupVersionResource) string {
	return fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
}
