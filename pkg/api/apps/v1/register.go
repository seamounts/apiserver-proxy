package appsv1

import (
	"context"
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/registry"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AppsV1Group struct{}

func NewAppsV1Group() *AppsV1Group {
	return &AppsV1Group{}
}

func (g *AppsV1Group) GroupName() string {
	return "apps"
}

func (g *AppsV1Group) GroupVersion() string {
	return "apps/v1"
}

func (g *AppsV1Group) Register(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	gv := schema.GroupVersion{Group: "apps", Version: "v1"}

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
			resource:        "deployments",
			singularName:    "deployment",
			namespaceScoped: true,
			shortNames:      []string{"deploy"},
			categories:      []string{"all"},
			objectType:      &appsv1.Deployment{},
			listObjectType:  &appsv1.DeploymentList{},
			storageWrapper:  wrapDeploymentStorage,
		},
		{
			resource:        "replicasets",
			singularName:    "replicaset",
			namespaceScoped: true,
			shortNames:      []string{"rs"},
			categories:      []string{"all"},
			objectType:      &appsv1.ReplicaSet{},
			listObjectType:  &appsv1.ReplicaSetList{},
		},
		{
			resource:        "daemonsets",
			singularName:    "daemonset",
			namespaceScoped: true,
			shortNames:      []string{"ds"},
			objectType:      &appsv1.DaemonSet{},
			listObjectType:  &appsv1.DaemonSetList{},
		},
		{
			resource:        "statefulsets",
			singularName:    "statefulset",
			namespaceScoped: true,
			shortNames:      []string{"sts"},
			objectType:      &appsv1.StatefulSet{},
			listObjectType:  &appsv1.StatefulSetList{},
		},
		{
			resource:        "controllerrevisions",
			singularName:    "controllerrevision",
			namespaceScoped: true,
			objectType:      &appsv1.ControllerRevision{},
			listObjectType:  &appsv1.ControllerRevisionList{},
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

func (g *AppsV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	return g.Register(r, factory)
}

type DeploymentStorage struct {
	registry.Storage
}

func wrapDeploymentStorage(s registry.Storage) registry.Storage {
	return &DeploymentStorage{Storage: s}
}

func (s *DeploymentStorage) New() runtime.Object {
	return &appsv1.Deployment{}
}

func (s *DeploymentStorage) Create(ctx context.Context, obj runtime.Object, createValidation registry.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("expected *appsv1.Deployment, got %T", obj)
	}

	if deployment.Namespace == "" {
		deployment.Namespace = "default"
	}

	if deployment.Spec.Replicas == nil {
		replicas := int32(1)
		deployment.Spec.Replicas = &replicas
	}

	if deployment.Spec.Selector == nil && len(deployment.Spec.Template.Labels) > 0 {
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: deployment.Spec.Template.Labels,
		}
	}

	return s.Storage.Create(ctx, deployment, createValidation, options)
}

func (s *DeploymentStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return s.Storage.Get(ctx, name, options)
}

func (s *DeploymentStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	return s.Storage.List(ctx, options)
}
