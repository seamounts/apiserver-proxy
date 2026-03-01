package v1

import (
	"context"
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PodStorage struct {
	registry.Storage
}

func NewPodStorage(s registry.Storage) registry.Storage {
	return &PodStorage{Storage: s}
}

func (s *PodStorage) New() runtime.Object {
	return &corev1.Pod{}
}

func (s *PodStorage) Create(ctx context.Context, obj runtime.Object, createValidation registry.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected *corev1.Pod, got %T", obj)
	}

	if pod.Namespace == "" {
		pod.Namespace = "default"
	}

	if pod.Spec.ServiceAccountName == "" {
		pod.Spec.ServiceAccountName = "default"
	}

	return s.Storage.Create(ctx, pod, createValidation, options)
}

func (s *PodStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return s.Storage.Get(ctx, name, options)
}

func (s *PodStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	return s.Storage.List(ctx, options)
}

var PodResource = ResourceStorage{
	SingularName:    "pod",
	NamespaceScoped: true,
	ShortNames:      []string{"po"},
	Categories:      []string{"all"},
	ObjectType:      &corev1.Pod{},
	ListObjectType:  &corev1.PodList{},
	StorageWrapper:  NewPodStorage,
}
