package api

import (
	"github.com/seamounts/apiserver-proxy/pkg/registry"
)

type GroupVersion interface {
	GroupVersion() string
	Register(registry *registry.ResourceRegistry, factory registry.StorageFactory) error
}

type APIRegistration struct {
	Groups []GroupVersion
}

func NewAPIRegistration() *APIRegistration {
	return &APIRegistration{
		Groups: make([]GroupVersion, 0),
	}
}

func (a *APIRegistration) RegisterGroup(gv GroupVersion) {
	a.Groups = append(a.Groups, gv)
}

func (a *APIRegistration) RegisterAll(registry *registry.ResourceRegistry, factory registry.StorageFactory) error {
	for _, gv := range a.Groups {
		if err := gv.Register(registry, factory); err != nil {
			return err
		}
	}
	return nil
}
