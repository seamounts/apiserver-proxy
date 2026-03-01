package v1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type EndpointsStorage struct {
	registry.Storage
}

func NewEndpointsStorage(s registry.Storage) registry.Storage {
	return &EndpointsStorage{Storage: s}
}

var EndpointsResource = ResourceStorage{
	SingularName:    "endpoints",
	NamespaceScoped: true,
	ShortNames:      []string{"ep"},
	ObjectType:      &corev1.Endpoints{},
	ListObjectType:  &corev1.EndpointsList{},
	StorageWrapper:  NewEndpointsStorage,
}
