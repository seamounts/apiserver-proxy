package batchv1

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	batchv1 "k8s.io/api/batch/v1"
)

type JobStorage struct {
	registry.Storage
}

func NewJobStorage(s registry.Storage) registry.Storage {
	return &JobStorage{Storage: s}
}

var JobResource = ResourceStorage{
	SingularName:    "job",
	NamespaceScoped: true,
	Categories:      []string{"all"},
	ObjectType:      &batchv1.Job{},
	ListObjectType:  &batchv1.JobList{},
	StorageWrapper:  NewJobStorage,
}
