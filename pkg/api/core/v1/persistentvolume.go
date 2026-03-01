package v1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type PersistentVolumeStorage struct {
	registry.Storage
}

func NewPersistentVolumeStorage(s registry.Storage) registry.Storage {
	return &PersistentVolumeStorage{Storage: s}
}

var PersistentVolumeResource = ResourceStorage{
	SingularName:    "persistentvolume",
	NamespaceScoped: false,
	ShortNames:      []string{"pv"},
	ObjectType:      &corev1.PersistentVolume{},
	ListObjectType:  &corev1.PersistentVolumeList{},
	StorageWrapper:  NewPersistentVolumeStorage,
}
