// Package api provides API resource registration and management.
// It defines the interfaces and managers for registering Kubernetes API resources.
package api

import (
	"fmt"

	"github.com/seamounts/apiserver-proxy/pkg/api/appsv1"
	"github.com/seamounts/apiserver-proxy/pkg/api/batchv1"
	"github.com/seamounts/apiserver-proxy/pkg/api/corev1"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
)

// GroupRegistrar is the interface for an API group registrar.
// Each API group (e.g., core/v1, apps/v1) implements this interface
// to register its resources.
type GroupRegistrar interface {
	// GroupName returns the API group name (e.g., "", "apps", "batch")
	GroupName() string
	// RegisterResources registers all resources in this group
	RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error
}

// APIManager manages the registration of API groups.
// It provides a centralized way to register multiple API groups.
type APIManager struct {
	groups []GroupRegistrar
}

// NewAPIManager creates a new APIManager instance.
func NewAPIManager() *APIManager {
	return &APIManager{
		groups: make([]GroupRegistrar, 0),
	}
}

// RegisterGroup adds an API group to the manager.
func (m *APIManager) RegisterGroup(group GroupRegistrar) {
	m.groups = append(m.groups, group)
}

// RegisterAll registers all managed API groups to the resource registry.
// It iterates through all registered groups and calls their RegisterResources method.
func (m *APIManager) RegisterAll(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
	for _, group := range m.groups {
		if err := group.RegisterResources(r, factory); err != nil {
			return fmt.Errorf("failed to register group %s: %v", group.GroupName(), err)
		}
	}
	return nil
}

// ListGroups returns the names of all registered API groups.
func (m *APIManager) ListGroups() []string {
	names := make([]string, 0, len(m.groups))
	for _, g := range m.groups {
		names = append(names, g.GroupName())
	}
	return names
}

// DefaultAPIManager returns an APIManager with all built-in API groups registered.
// This includes core/v1, apps/v1, and batch/v1.
func DefaultAPIManager() *APIManager {
	m := NewAPIManager()
	m.RegisterGroup(corev1.NewCoreV1Group())
	m.RegisterGroup(appsv1.NewAppsV1Group())
	m.RegisterGroup(batchv1.NewBatchV1Group())
	return m
}
