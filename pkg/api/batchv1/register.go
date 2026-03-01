package batchv1

import (
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type BatchV1Group struct{}

func NewBatchV1Group() *BatchV1Group {
	return &BatchV1Group{}
}

func (g *BatchV1Group) GroupName() string {
	return "batch"
}

func (g *BatchV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	gv := schema.GroupVersion{Group: "batch", Version: "v1"}

	resources := map[string]types.APIResource{
		"jobs": JobResource,
	}

	for resourceName, res := range resources {
		gvr := gv.WithResource(resourceName)
		builder := registry.NewResourceBuilder(gvr).
			SingularName(res.SingularName).
			NamespaceScoped(res.NamespaceScoped).
			ShortNames(res.ShortNames...).
			Categories(res.Categories...).
			ObjectType(res.ObjectType).
			ListObjectType(res.ListObjectType).
			StorageFactory(factory)

		if res.StorageWrapper != nil {
			info, err := builder.Build()
			if err != nil {
				return fmt.Errorf("failed to build resource %s: %v", resourceName, err)
			}
			info.Storage = res.StorageWrapper(info.Storage)
		}

		if err := r.Register(builder); err != nil {
			return fmt.Errorf("failed to register %s: %v", resourceName, err)
		}
	}

	return nil
}
