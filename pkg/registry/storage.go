// Package registry provides interfaces and implementations for resource storage and registration.
// It defines the core storage interfaces that all resource handlers must implement,
// as well as the resource registry for managing registered resources.
package registry

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// ValidateObjectFunc is a function type for validating objects.
// It is called during create/update operations to validate the object before storage.
type ValidateObjectFunc func(ctx context.Context, obj runtime.Object) error

// ValidateObjectUpdateFunc is a function type for validating object updates.
// It receives both the new and old object for comparison during updates.
type ValidateObjectUpdateFunc func(ctx context.Context, newObj, oldObj runtime.Object) error

// UpdatedObjectInfo provides information about an updated object.
// It is used during update operations to determine the new object state.
type UpdatedObjectInfo interface {
	// UpdatedObject returns the updated object based on the old object state.
	UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error)
}

// Storage is the core interface for resource storage operations.
// All resource handlers must implement this interface to support CRUD operations.
type Storage interface {
	// New returns a new empty object of the resource type.
	New() runtime.Object
	// Create creates a new resource object.
	Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error)
	// Get retrieves a resource object by name.
	Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
	// List retrieves a list of resource objects.
	List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error)
	// Update updates an existing resource object.
	Update(ctx context.Context, name string, objInfo UpdatedObjectInfo, createValidation ValidateObjectFunc, updateValidation ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error)
	// Delete deletes a resource object by name.
	Delete(ctx context.Context, name string, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error)
	// DeleteCollection deletes a collection of resource objects.
	DeleteCollection(ctx context.Context, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metav1.ListOptions) (runtime.Object, error)
	// Watch watches for changes to resource objects.
	Watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error)
}

// StandardStorage extends Storage with additional metadata methods.
type StandardStorage interface {
	Storage
	// NamespaceScoped returns true if the resource is namespaced.
	NamespaceScoped() bool
	// GetSingularName returns the singular name of the resource.
	GetSingularName() string
}

// StorageFactory is an interface for creating storage instances.
// It is used to create storage for different resources.
type StorageFactory interface {
	// NewStorage creates a new storage instance for the given resource.
	NewStorage(gvr schema.GroupVersionResource) (Storage, error)
}

// StorageFactoryFunc is a function type that implements StorageFactory.
type StorageFactoryFunc func(gvr schema.GroupVersionResource) (Storage, error)

// NewStorage implements StorageFactory interface.
func (f StorageFactoryFunc) NewStorage(gvr schema.GroupVersionResource) (Storage, error) {
	return f(gvr)
}
