package registry

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SubresourceStorage is the interface for subresource storage operations.
// Subresources like status, scale, etc. implement this interface.
type SubresourceStorage interface {
	// New returns a new empty object of the subresource type.
	New() runtime.Object

	// Get retrieves the subresource for a parent resource.
	Get(ctx context.Context, parentName string, options *metav1.GetOptions) (runtime.Object, error)

	// Update updates the subresource for a parent resource.
	Update(ctx context.Context, parentName string, objInfo UpdatedObjectInfo, createValidation ValidateObjectFunc, updateValidation ValidateObjectUpdateFunc, options *metav1.UpdateOptions) (runtime.Object, bool, error)
}

// StatusSubresourceStorage is a specialized interface for status subresources.
type StatusSubresourceStorage interface {
	SubresourceStorage
	// NamespaceScoped returns whether the parent resource is namespace scoped.
	NamespaceScoped() bool
}

// ScaleSubresourceStorage is a specialized interface for scale subresources.
type ScaleSubresourceStorage interface {
	SubresourceStorage
	// NamespaceScoped returns whether the parent resource is namespace scoped.
	NamespaceScoped() bool
}

// LogSubresourceStorage is the interface for log subresources (read-only).
type LogSubresourceStorage interface {
	// Get retrieves logs for a pod/container.
	Get(ctx context.Context, parentName string, options *corev1.PodLogOptions) (runtime.Object, error)
}

// ExecSubresourceStorage is the interface for exec subresources.
type ExecSubresourceStorage interface {
	// Connect connects to the exec endpoint.
	Connect(ctx context.Context, parentName string, options *corev1.PodExecOptions) (runtime.Object, error)
}

// SubresourceInfo contains metadata for a subresource.
type SubresourceInfo struct {
	// Name is the subresource name (e.g., "status", "scale")
	Name string
	// GVR is the full GroupVersionResource including subresource
	GVR string
	// Storage is the storage implementation
	Storage SubresourceStorage
	// Verbs are the supported verbs (e.g., "get", "update")
	Verbs []string
	// ParentResource is the parent resource name
	ParentResource string
}

// SubresourceBuilder provides a fluent API for building subresource configurations.
type SubresourceBuilder struct {
	info *SubresourceInfo
}

// NewSubresourceBuilder creates a new SubresourceBuilder.
func NewSubresourceBuilder(name string) *SubresourceBuilder {
	return &SubresourceBuilder{
		info: &SubresourceInfo{
			Name:  name,
			Verbs: []string{"get", "update"},
		},
	}
}

// Storage sets the storage implementation.
func (b *SubresourceBuilder) Storage(s SubresourceStorage) *SubresourceBuilder {
	b.info.Storage = s
	return b
}

// Verbs sets the supported verbs.
func (b *SubresourceBuilder) Verbs(verbs ...string) *SubresourceBuilder {
	b.info.Verbs = verbs
	return b
}

// ParentResource sets the parent resource name.
func (b *SubresourceBuilder) ParentResource(resource string) *SubresourceBuilder {
	b.info.ParentResource = resource
	return b
}

// Build returns the SubresourceInfo.
func (b *SubresourceBuilder) Build() *SubresourceInfo {
	return b.info
}
