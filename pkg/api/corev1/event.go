package v1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type EventStorage struct {
	registry.Storage
}

func NewEventStorage(s registry.Storage) registry.Storage {
	return &EventStorage{Storage: s}
}

var EventResource = ResourceStorage{
	SingularName:    "event",
	NamespaceScoped: true,
	ShortNames:      []string{"ev"},
	ObjectType:      &corev1.Event{},
	ListObjectType:  &corev1.EventList{},
	StorageWrapper:  NewEventStorage,
}
