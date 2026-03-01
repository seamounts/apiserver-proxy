package corev1

import (
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
)

type CoreV1Group struct{}

func NewCoreV1Group() *CoreV1Group {
	return &CoreV1Group{}
}

func (g *CoreV1Group) GroupName() string {
	return ""
}

func (g *CoreV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	resources := []types.APIResource{
		PodResource,
		ServiceResource,
		ConfigMapResource,
		SecretResource,
		NamespaceResource,
		NodeResource,
	}

	for _, res := range resources {
		builder := registry.NewResourceBuilder(res.GVR()).
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
				return fmt.Errorf("failed to build resource %s: %v", res.Resource, err)
			}
			info.Storage = res.StorageWrapper(info.Storage)

			for _, sr := range res.Subresources {
				var srStorage registry.SubresourceStorage
				switch sr.Name {
				case "status":
					srStorage = NewPodStatusStorage(info.Storage)
				default:
					continue
				}

				srInfo := &registry.SubresourceInfo{
					Name:           sr.Name,
					Storage:        srStorage,
					Verbs:          sr.Verbs,
					ParentResource: res.Resource,
				}
				builder.Subresource(srInfo)
			}
		}

		if err := r.Register(builder); err != nil {
			return fmt.Errorf("failed to register %s: %v", res.Resource, err)
		}
	}

	return nil
}
