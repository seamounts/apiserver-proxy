package appsv1

import (
	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	appsv1 "k8s.io/api/apps/v1"
)

type ReplicaSetStorage struct {
	registry.Storage
}

func NewReplicaSetStorage(s registry.Storage) registry.Storage {
	return &ReplicaSetStorage{Storage: s}
}

var ReplicaSetResource = types.APIResource{
	Group:           "apps",
	Version:         "v1",
	Kind:            "ReplicaSet",
	Resource:        "replicasets",
	SingularName:    "replicaset",
	NamespaceScoped: true,
	ShortNames:      []string{"rs"},
	Categories:      []string{"all"},
	ObjectType:      &appsv1.ReplicaSet{},
	ListObjectType:  &appsv1.ReplicaSetList{},
	StorageWrapper:  NewReplicaSetStorage,
}
