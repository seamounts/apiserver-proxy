package corev1

import (
	"context"
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/types"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	if err := s.validatePod(pod); err != nil {
		return nil, err
	}

	s.setPodDefaults(pod)

	if createValidation != nil {
		if err := createValidation(ctx, pod); err != nil {
			return nil, err
		}
	}

	return s.Storage.Create(ctx, pod, createValidation, options)
}

func (s *PodStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, err := s.Storage.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return obj, nil
	}

	s.sanitizePod(pod)

	return pod, nil
}

func (s *PodStorage) List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error) {
	obj, err := s.Storage.List(ctx, options)
	if err != nil {
		return nil, err
	}

	podList, ok := obj.(*corev1.PodList)
	if !ok {
		return obj, nil
	}

	for i := range podList.Items {
		s.sanitizePod(&podList.Items[i])
	}

	return podList, nil
}

func (s *PodStorage) Update(ctx context.Context, name string, objInfo registry.UpdatedObjectInfo, createValidation registry.ValidateObjectFunc, updateValidation registry.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	oldObj, err := s.Storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		return nil, false, fmt.Errorf("expected *corev1.Pod, got %T", newObj)
	}

	if err := s.validatePodUpdate(oldObj.(*corev1.Pod), newPod); err != nil {
		return nil, false, err
	}

	s.setPodDefaults(newPod)

	return s.Storage.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

func (s *PodStorage) Delete(ctx context.Context, name string, deleteValidation registry.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	obj, err := s.Storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return s.Storage.Delete(ctx, name, deleteValidation, options)
	}

	if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
		for _, c := range pod.Status.ContainerStatuses {
			if c.Ready && c.State.Running != nil {
				fmt.Printf("Warning: deleting pod %s with running container %s\n", name, c.Name)
			}
		}
	}

	return s.Storage.Delete(ctx, name, deleteValidation, options)
}

func (s *PodStorage) validatePod(pod *corev1.Pod) error {
	if len(pod.Spec.Containers) == 0 {
		return field.Invalid(
			field.NewPath("spec", "containers"),
			nil,
			"must specify at least one container",
		)
	}

	containerNames := make(map[string]bool)
	for i, container := range pod.Spec.Containers {
		if container.Name == "" {
			return field.Invalid(
				field.NewPath("spec", "containers").Index(i).Child("name"),
				container.Name,
				"container name must not be empty",
			)
		}
		if containerNames[container.Name] {
			return field.Invalid(
				field.NewPath("spec", "containers").Index(i).Child("name"),
				container.Name,
				"duplicate container name",
			)
		}
		containerNames[container.Name] = true

		if container.Image == "" {
			return field.Invalid(
				field.NewPath("spec", "containers").Index(i).Child("image"),
				container.Image,
				"container image must not be empty",
			)
		}
	}

	return nil
}

func (s *PodStorage) validatePodUpdate(oldPod, newPod *corev1.Pod) error {
	if oldPod == nil {
		return nil
	}

	if oldPod.Spec.ServiceAccountName != newPod.Spec.ServiceAccountName {
		return field.Forbidden(
			field.NewPath("spec", "serviceAccountName"),
			"cannot change serviceAccountName",
		)
	}

	return nil
}

func (s *PodStorage) setPodDefaults(pod *corev1.Pod) {
	if pod.Namespace == "" {
		pod.Namespace = "default"
	}

	if pod.Spec.ServiceAccountName == "" {
		pod.Spec.ServiceAccountName = "default"
	}

	if pod.Spec.RestartPolicy == "" {
		pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	}

	if pod.Spec.DNSPolicy == "" {
		pod.Spec.DNSPolicy = corev1.DNSClusterFirst
	}

	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.TerminationMessagePolicy == "" {
			container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
		}
		if container.ImagePullPolicy == "" {
			if len(container.Image) > 0 && container.Image[len(container.Image)-1] == ':' {
				container.ImagePullPolicy = corev1.PullAlways
			} else {
				container.ImagePullPolicy = corev1.PullIfNotPresent
			}
		}
	}

	for i := range pod.Spec.InitContainers {
		container := &pod.Spec.InitContainers[i]
		if container.TerminationMessagePolicy == "" {
			container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
		}
		if container.ImagePullPolicy == "" {
			if len(container.Image) > 0 && container.Image[len(container.Image)-1] == ':' {
				container.ImagePullPolicy = corev1.PullAlways
			} else {
				container.ImagePullPolicy = corev1.PullIfNotPresent
			}
		}
	}

	if pod.Spec.SecurityContext == nil {
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}

	if pod.Spec.SchedulerName == "" {
		pod.Spec.SchedulerName = "default-scheduler"
	}
}

func (s *PodStorage) sanitizePod(pod *corev1.Pod) {
	if pod.APIVersion == "" {
		pod.APIVersion = "v1"
	}
	if pod.Kind == "" {
		pod.Kind = "Pod"
	}
}

var PodResource = types.APIResource{
	Group:           "",
	Version:         "v1",
	Kind:            "Pod",
	Resource:        "pods",
	SingularName:    "pod",
	NamespaceScoped: true,
	ShortNames:      []string{"po"},
	Categories:      []string{"all"},
	ObjectType:      &corev1.Pod{},
	ListObjectType:  &corev1.PodList{},
	StorageWrapper:  NewPodStorage,
}
