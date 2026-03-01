package corev1

import (
	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type PodStorage struct {
	registry.Storage
}

func NewPodStorage(s registry.Storage) registry.Storage {
	return &PodStorage{Storage: s}
}

var PodResource = types.APIResource{
	SingularName:    "pod",
	NamespaceScoped: true,
	ShortNames:      []string{"po"},
	Categories:      []string{"all"},
	ObjectType:      &corev1.Pod{},
	ListObjectType:  &corev1.PodList{},
	StorageWrapper:  NewPodStorage,
}
