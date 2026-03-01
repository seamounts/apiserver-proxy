package v1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type SecretStorage struct {
	registry.Storage
}

func NewSecretStorage(s registry.Storage) registry.Storage {
	return &SecretStorage{Storage: s}
}

var SecretResource = ResourceStorage{
	SingularName:    "secret",
	NamespaceScoped: true,
	ShortNames:      []string{"sec"},
	ObjectType:      &corev1.Secret{},
	ListObjectType:  &corev1.SecretList{},
	StorageWrapper:  NewSecretStorage,
}
