package registry

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PodStorage struct {
	Storage
}

func NewPodStorage(underlyingStorage Storage) *PodStorage {
	return &PodStorage{
		Storage: underlyingStorage,
	}
}

func (s *PodStorage) New() runtime.Object {
	return &corev1.Pod{}
}

func (s *PodStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	return s.Storage.List(ctx, options)
}

func (s *PodStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return s.Storage.Get(ctx, name, options)
}

func (s *PodStorage) Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected *corev1.Pod, got %T", obj)
	}

	if pod.ObjectMeta.Namespace == "" {
		pod.ObjectMeta.Namespace = "default"
	}

	return s.Storage.Create(ctx, pod, createValidation, options)
}

type DeploymentStorage struct {
	Storage
}

func NewDeploymentStorage(underlyingStorage Storage) *DeploymentStorage {
	return &DeploymentStorage{
		Storage: underlyingStorage,
	}
}

func (s *DeploymentStorage) New() runtime.Object {
	return &appsv1.Deployment{}
}

func (s *DeploymentStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	return s.Storage.List(ctx, options)
}

func (s *DeploymentStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return s.Storage.Get(ctx, name, options)
}

func (s *DeploymentStorage) Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("expected *appsv1.Deployment, got %T", obj)
	}

	if deployment.ObjectMeta.Namespace == "" {
		deployment.ObjectMeta.Namespace = "default"
	}

	return s.Storage.Create(ctx, deployment, createValidation, options)
}

type CustomResourceStorage struct {
	Storage
	gvr schema.GroupVersionResource
}

func NewCustomResourceStorage(underlyingStorage Storage, gvr schema.GroupVersionResource) *CustomResourceStorage {
	return &CustomResourceStorage{
		Storage: underlyingStorage,
		gvr:     gvr,
	}
}

func (s *CustomResourceStorage) New() runtime.Object {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       s.gvr.Resource,
			APIVersion: s.gvr.GroupVersion().String(),
		},
	}
}

func RegisterCoreResources(builder *RESTStorageBuilder, storageFactory StorageFactory) error {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podStorage, err := storageFactory.NewStorage(podGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "pods", NewPodStorage(podStorage), "pod", true)

	serviceGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	serviceStorage, err := storageFactory.NewStorage(serviceGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "services", serviceStorage, "service", true)

	configMapGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	configMapStorage, err := storageFactory.NewStorage(configMapGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "configmaps", configMapStorage, "configmap", true)

	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	secretStorage, err := storageFactory.NewStorage(secretGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "secrets", secretStorage, "secret", true)

	return nil
}

func RegisterAppsResources(builder *RESTStorageBuilder, storageFactory StorageFactory) error {
	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	deploymentStorage, err := storageFactory.NewStorage(deploymentGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "deployments", NewDeploymentStorage(deploymentStorage), "deployment", true)

	replicaSetGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	replicaSetStorage, err := storageFactory.NewStorage(replicaSetGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "replicasets", replicaSetStorage, "replicaset", true)

	daemonSetGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	daemonSetStorage, err := storageFactory.NewStorage(daemonSetGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "daemonsets", daemonSetStorage, "daemonset", true)

	statefulSetGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	statefulSetStorage, err := storageFactory.NewStorage(statefulSetGVR)
	if err != nil {
		return err
	}
	builder.AddStorage("v1", "statefulsets", statefulSetStorage, "statefulset", true)

	return nil
}

func RegisterCustomResource(builder *RESTStorageBuilder, gvr schema.GroupVersionResource, storageFactory StorageFactory, singularName string, namespaceScoped bool) error {
	storage, err := storageFactory.NewStorage(gvr)
	if err != nil {
		return err
	}

	customStorage := NewCustomResourceStorage(storage, gvr)
	builder.AddStorage(gvr.Version, gvr.Resource, customStorage, singularName, namespaceScoped)

	return nil
}
