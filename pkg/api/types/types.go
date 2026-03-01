// Package types provides common type definitions for API resources.
package types

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// APIResource represents a Kubernetes API resource definition.
// It contains all the metadata needed to register and handle a resource.
type APIResource struct {
	// Group is the API group name (e.g., "apps", "batch", "" for core)
	Group string
	// Version is the API version (e.g., "v1", "v1beta1")
	Version string
	// Kind is the resource kind name (e.g., "Pod", "Deployment")
	Kind string
	// Resource is the plural resource name used in URLs (e.g., "pods", "deployments")
	Resource string
	// SingularName is the singular resource name (e.g., "pod", "deployment")
	SingularName string
	// NamespaceScoped indicates if the resource is namespaced
	NamespaceScoped bool
	// ShortNames are short aliases for the resource (e.g., "po" for pods)
	ShortNames []string
	// Categories are groups of resources this belongs to (e.g., "all")
	Categories []string
	// ObjectType is the Go type for the resource object
	ObjectType runtime.Object
	// ListObjectType is the Go type for the resource list object
	ListObjectType runtime.Object
	// StorageWrapper is an optional function to wrap the storage implementation
	StorageWrapper func(registry.Storage) registry.Storage
}

// GVR returns the GroupVersionResource for this API resource.
func (r *APIResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Resource,
	}
}

// GVK returns the GroupVersionKind for this API resource.
func (r *APIResource) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

// DefaultStorageWrapper is a no-op storage wrapper that returns the storage unchanged.
var DefaultStorageWrapper = func(s registry.Storage) registry.Storage {
	return s
}
