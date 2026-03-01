package corev1

import (
	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type ConfigMapStorage struct {
	registry.Storage
}

func NewConfigMapStorage(s registry.Storage) registry.Storage {
	return &ConfigMapStorage{Storage: s}
}

var ConfigMapResource = types.APIResource{
	SingularName:    "configmap",
	NamespaceScoped: true,
	ShortNames:      []string{"cm"},
	ObjectType:      &corev1.ConfigMap{},
	ListObjectType:  &corev1.ConfigMapList{},
	StorageWrapper:  NewConfigMapStorage,
}
