package types

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
)

type APIResource struct {
	SingularName    string
	NamespaceScoped bool
	ShortNames      []string
	Categories      []string
	ObjectType      runtime.Object
	ListObjectType  runtime.Object
	StorageWrapper  func(registry.Storage) registry.Storage
}

var DefaultStorageWrapper = func(s registry.Storage) registry.Storage {
	return s
}
