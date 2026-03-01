package corev1

import (
	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type ServiceStorage struct {
	registry.Storage
}

func NewServiceStorage(s registry.Storage) registry.Storage {
	return &ServiceStorage{Storage: s}
}

var ServiceResource = types.APIResource{
	Group:           "",
	Version:         "v1",
	Kind:            "Service",
	Resource:        "services",
	SingularName:    "service",
	NamespaceScoped: true,
	ShortNames:      []string{"svc"},
	Categories:      []string{"all"},
	ObjectType:      &corev1.Service{},
	ListObjectType:  &corev1.ServiceList{},
	StorageWrapper:  NewServiceStorage,
}
