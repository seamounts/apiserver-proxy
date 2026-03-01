package registry

import (
	"context"

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

type StorageFactory interface {
	NewStorage(gvr schema.GroupVersionResource) (Storage, error)
}

type StorageFactoryFunc func(gvr schema.GroupVersionResource) (Storage, error)

func (f StorageFactoryFunc) NewStorage(gvr schema.GroupVersionResource) (Storage, error) {
	return f(gvr)
}
