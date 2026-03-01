package appsv1

import (
	"context"
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type DeploymentStorage struct {
	registry.Storage
}

func NewDeploymentStorage(s registry.Storage) registry.Storage {
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

var DeploymentResource = types.APIResource{
	SingularName:    "deployment",
	NamespaceScoped: true,
	ShortNames:      []string{"deploy"},
	Categories:      []string{"all"},
	ObjectType:      &appsv1.Deployment{},
	ListObjectType:  &appsv1.DeploymentList{},
	StorageWrapper:  NewDeploymentStorage,
}
