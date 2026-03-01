package types

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type APIResource struct {
	Group           string
	Version         string
	Kind            string
	Resource        string
	SingularName    string
	NamespaceScoped bool
	ShortNames      []string
	Categories      []string
	ObjectType      runtime.Object
	ListObjectType  runtime.Object
	StorageWrapper  func(registry.Storage) registry.Storage
}

func (r *APIResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Resource,
	}
}

func (r *APIResource) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

var DefaultStorageWrapper = func(s registry.Storage) registry.Storage {
	return s
}
