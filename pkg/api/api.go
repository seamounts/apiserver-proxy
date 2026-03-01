package api

import (
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/appsv1"
	"github.com/seamounts/apiserver-proxy/pkg/api/batchv1"
	"github.com/seamounts/apiserver-proxy/pkg/api/corev1"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
)

type GroupRegistrar interface {
	GroupName() string
	RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error
}

type APIManager struct {
	groups []GroupRegistrar
}

func NewAPIManager() *APIManager {
	return &APIManager{
		groups: make([]GroupRegistrar, 0),
	}
}

func (m *APIManager) RegisterGroup(group GroupRegistrar) {
	m.groups = append(m.groups, group)
}

func (m *APIManager) RegisterAll(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	for _, group := range m.groups {
		if err := group.RegisterResources(r, factory); err != nil {
			return fmt.Errorf("failed to register group %s: %v", group.GroupName(), err)
		}
	}
	return nil
}

func (m *APIManager) ListGroups() []string {
	names := make([]string, 0, len(m.groups))
	for _, g := range m.groups {
		names = append(names, g.GroupName())
	}
	return names
}

func DefaultAPIManager() *APIManager {
	m := NewAPIManager()
	m.RegisterGroup(corev1.NewCoreV1Group())
	m.RegisterGroup(appsv1.NewAppsV1Group())
	m.RegisterGroup(batchv1.NewBatchV1Group())
	return m
}
