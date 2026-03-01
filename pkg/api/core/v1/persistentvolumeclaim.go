package v1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type PersistentVolumeClaimStorage struct {
	registry.Storage
}

func NewPersistentVolumeClaimStorage(s registry.Storage) registry.Storage {
	return &PersistentVolumeClaimStorage{Storage: s}
}

var PersistentVolumeClaimResource = ResourceStorage{
	SingularName:    "persistentvolumeclaim",
	NamespaceScoped: true,
	ShortNames:      []string{"pvc"},
	ObjectType:      &corev1.PersistentVolumeClaim{},
	ListObjectType:  &corev1.PersistentVolumeClaimList{},
	StorageWrapper:  NewPersistentVolumeClaimStorage,
}
