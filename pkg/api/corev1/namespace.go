package corev1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type NamespaceStorage struct {
	registry.Storage
}

func NewNamespaceStorage(s registry.Storage) registry.Storage {
	return &NamespaceStorage{Storage: s}
}

var NamespaceResource = ResourceStorage{
	SingularName:    "namespace",
	NamespaceScoped: false,
	ShortNames:      []string{"ns"},
	ObjectType:      &corev1.Namespace{},
	ListObjectType:  &corev1.NamespaceList{},
	StorageWrapper:  NewNamespaceStorage,
}
