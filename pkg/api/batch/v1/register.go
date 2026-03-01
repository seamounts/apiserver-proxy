package v1

import (
	"context"
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/registry"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type BatchV1Group struct{}

func NewBatchV1Group() *BatchV1Group {
	return &BatchV1Group{}
}

func (g *BatchV1Group) GroupName() string {
	return "batch"
}

func (g *BatchV1Group) GroupVersion() string {
	return "batch/v1"
}

func (g *BatchV1Group) Register(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	gv := schema.GroupVersion{Group: "batch", Version: "v1"}

	resources := []struct {
		resource        string
		singularName    string
		namespaceScoped bool
		shortNames      []string
		categories      []string
		objectType      runtime.Object
		listObjectType  runtime.Object
		storageWrapper  func(registry.Storage) registry.Storage
	}{
		{
			resource:        "jobs",
			singularName:    "job",
			namespaceScoped: true,
			categories:      []string{"all"},
			objectType:      &batchv1.Job{},
			listObjectType:  &batchv1.JobList{},
			storageWrapper:  wrapJobStorage,
		},
		{
			resource:        "cronjobs",
			singularName:    "cronjob",
			namespaceScoped: true,
			shortNames:      []string{"cj"},
			objectType:      &batchv1.CronJob{},
			listObjectType:  &batchv1.CronJobList{},
		},
	}

	for _, res := range resources {
		gvr := gv.WithResource(res.resource)
		builder := registry.NewResourceBuilder(gvr).
			SingularName(res.singularName).
			NamespaceScoped(res.namespaceScoped).
			ShortNames(res.shortNames...).
			Categories(res.categories...).
			ObjectType(res.objectType).
			ListObjectType(res.listObjectType).
			StorageFactory(factory)

		if res.storageWrapper != nil {
			info, err := builder.Build()
			if err != nil {
				return fmt.Errorf("failed to build resource %s: %v", res.resource, err)
			}
			info.Storage = res.storageWrapper(info.Storage)
		}

		if err := r.Register(builder); err != nil {
			return fmt.Errorf("failed to register %s: %v", res.resource, err)
		}
	}

	return nil
}

func (g *BatchV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	return g.Register(r, factory)
}

type JobStorage struct {
	registry.Storage
}

func wrapJobStorage(s registry.Storage) registry.Storage {
	return &JobStorage{Storage: s}
}

func (s *JobStorage) New() runtime.Object {
	return &batchv1.Job{}
}

func (s *JobStorage) Create(ctx context.Context, obj runtime.Object, createValidation registry.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return nil, fmt.Errorf("expected *batchv1.Job, got %T", obj)
	}

	if job.Namespace == "" {
		job.Namespace = "default"
	}

	return s.Storage.Create(ctx, job, createValidation, options)
}

func (s *JobStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return s.Storage.Get(ctx, name, options)
}

func (s *JobStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	return s.Storage.List(ctx, options)
}
