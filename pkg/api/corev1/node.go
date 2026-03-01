package corev1

import (
	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
)

type NodeStorage struct {
	registry.Storage
}

func NewNodeStorage(s registry.Storage) registry.Storage {
	return &NodeStorage{Storage: s}
}

var NodeResource = types.APIResource{
	Group:           "",
	Version:         "v1",
	Kind:            "Node",
	Resource:        "nodes",
	SingularName:    "node",
	NamespaceScoped: false,
	ShortNames:      []string{"no"},
	ObjectType:      &corev1.Node{},
	ListObjectType:  &corev1.NodeList{},
	StorageWrapper:  NewNodeStorage,
}
