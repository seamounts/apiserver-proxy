package corev1

import (
	"context"
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PodStatusStorage implements the status subresource for Pod.
type PodStatusStorage struct {
	registry.Storage
	parentStorage registry.Storage
}

// NewPodStatusStorage creates a new PodStatusStorage.
func NewPodStatusStorage(parentStorage registry.Storage) registry.SubresourceStorage {
	return &PodStatusStorage{
		Storage:       parentStorage,
		parentStorage: parentStorage,
	}
}

// New returns a new Pod object.
func (s *PodStatusStorage) New() runtime.Object {
	return &corev1.Pod{}
}

// Get retrieves the pod status.
func (s *PodStatusStorage) Get(ctx context.Context, parentName string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, err := s.parentStorage.Get(ctx, parentName, options)
	if err != nil {
		return nil, err
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("expected *corev1.Pod, got %T", obj)
	}

	statusPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            pod.Name,
			Namespace:       pod.Namespace,
			UID:             pod.UID,
			ResourceVersion: pod.ResourceVersion,
			Labels:          pod.Labels,
			Annotations:     pod.Annotations,
		},
		Status: pod.Status,
	}

	return statusPod, nil
}

// Update updates the pod status.
func (s *PodStatusStorage) Update(ctx context.Context, parentName string, objInfo registry.UpdatedObjectInfo, createValidation registry.ValidateObjectFunc, updateValidation registry.ValidateObjectUpdateFunc, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	oldObj, err := s.parentStorage.Get(ctx, parentName, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	oldPod, ok := oldObj.(*corev1.Pod)
	if !ok {
		return nil, false, fmt.Errorf("expected *corev1.Pod, got %T", oldObj)
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		return nil, false, fmt.Errorf("expected *corev1.Pod, got %T", newObj)
	}

	updatedPod := oldPod.DeepCopy()
	updatedPod.Status = newPod.Status
	updatedPod.ResourceVersion = newPod.ResourceVersion

	updatedInfo := &simpleUpdatedObjectInfo{obj: updatedPod}

	return s.parentStorage.Update(ctx, parentName, updatedInfo, createValidation, updateValidation, false, options)
}

// simpleUpdatedObjectInfo is a simple implementation of UpdatedObjectInfo.
type simpleUpdatedObjectInfo struct {
	obj runtime.Object
}

func (i *simpleUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	return i.obj, nil
}
