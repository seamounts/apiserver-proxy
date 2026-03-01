package appsv1

import (
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AppsV1Group struct{}

func NewAppsV1Group() *AppsV1Group {
	return &AppsV1Group{}
}

func (g *AppsV1Group) GroupName() string {
	return "apps"
}

func (g *AppsV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	gv := schema.GroupVersion{Group: "apps", Version: "v1"}

	resources := map[string]types.APIResource{
		"deployments":  DeploymentResource,
		"replicasets": ReplicaSetResource,
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
