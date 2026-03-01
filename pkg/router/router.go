// Package router provides HTTP routing functionality for the API server.
// It implements a hierarchical router structure for Kubernetes-style API paths.
package router

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Route represents a single HTTP route.
type Route struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, etc.)
	Method string
	// Path is the URL path pattern
	Path string
	// Handler is the HTTP handler function
	Handler http.HandlerFunc
	// GVR is the GroupVersionResource for this route
	GVR schema.GroupVersionResource
	// Verb is the API verb (get, list, create, update, delete)
	Verb string
	// SubResource is the subresource name (if applicable)
	SubResource string
}

// Router is the main router that manages all routes.
type Router struct {
	routes         []*Route
	groups         map[string]*GroupRouter
	parameterCodec ParameterCodec
}

// GroupRouter manages routes for an API group.
type GroupRouter struct {
	group    string
	versions map[string]*VersionRouter
}

// VersionRouter manages routes for an API group-version.
type VersionRouter struct {
	groupVersion schema.GroupVersion
	resources    map[string]*ResourceRouter
}

// ResourceRouter manages routes for a specific resource.
type ResourceRouter struct {
	gvr       schema.GroupVersionResource
	storage   interface{}
	subrouter map[string]*SubresourceRouter
}

// SubresourceRouter manages routes for a subresource.
type SubresourceRouter struct {
	gvr         schema.GroupVersionResource
	subresource string
	storage     interface{}
}

// ParameterCodec is an interface for encoding/decoding request parameters.
type ParameterCodec interface {
	DecodeParameters(req *http.Request, into interface{}) error
	EncodeParameters(obj interface{}, req *http.Request) error
}

// NewRouter creates a new Router instance.
func NewRouter() *Router {
	return &Router{
		routes: make([]*Route, 0),
		groups: make(map[string]*GroupRouter),
	}
}

// AddRoute adds a route to the router.
func (r *Router) AddRoute(route *Route) {
	r.routes = append(r.routes, route)
}

// Routes returns all registered routes.
func (r *Router) Routes() []*Route {
	return r.routes
}

// Match finds a route that matches the given HTTP request.
func (r *Router) Match(req *http.Request) (*Route, bool) {
	for _, route := range r.routes {
		if route.Method == req.Method && matchPath(route.Path, req.URL.Path) {
			return route, true
		}
	}
	return nil, false
}

// matchPath checks if a path matches a pattern.
// Pattern can contain placeholders like {name} which match any value.
func matchPath(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			continue
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// Group returns or creates a GroupRouter for the given API group.
func (r *Router) Group(group string) *GroupRouter {
	if _, exists := r.groups[group]; !exists {
		r.groups[group] = &GroupRouter{
			group:    group,
			versions: make(map[string]*VersionRouter),
		}
	}
	return r.groups[group]
}

// Version returns or creates a VersionRouter for the given API version.
func (g *GroupRouter) Version(version string) *VersionRouter {
	if _, exists := g.versions[version]; !exists {
		g.versions[version] = &VersionRouter{
			groupVersion: schema.GroupVersion{Group: g.group, Version: version},
			resources:    make(map[string]*ResourceRouter),
		}
	}
	return g.versions[version]
}

// Resource returns or creates a ResourceRouter for the given resource.
func (v *VersionRouter) Resource(resource string, storage interface{}) *ResourceRouter {
	if _, exists := v.resources[resource]; !exists {
		v.resources[resource] = &ResourceRouter{
			gvr: schema.GroupVersionResource{
				Group:    v.groupVersion.Group,
				Version:  v.groupVersion.Version,
				Resource: resource,
			},
			storage:   storage,
			subrouter: make(map[string]*SubresourceRouter),
		}
	}
	return v.resources[resource]
}

// Subresource returns or creates a SubresourceRouter for the given subresource.
func (r *ResourceRouter) Subresource(subresource string, storage interface{}) *SubresourceRouter {
	if _, exists := r.subrouter[subresource]; !exists {
		r.subrouter[subresource] = &SubresourceRouter{
			gvr: schema.GroupVersionResource{
				Group:    r.gvr.Group,
				Version:  r.gvr.Version,
				Resource: r.gvr.Resource + "/" + subresource,
			},
			subresource: subresource,
			storage:     storage,
		}
	}
	return r.subrouter[subresource]
}

// PathResolver resolves API paths to GVR and other components.
type PathResolver struct {
	groupPrefix string
}

// NewPathResolver creates a new PathResolver.
func NewPathResolver(groupPrefix string) *PathResolver {
	return &PathResolver{
		groupPrefix: groupPrefix,
	}
}

// ParsePath parses an API path into its components.
// Returns the GVR, resource name, subresource, and any error.
func (r *PathResolver) ParsePath(path string) (gvr schema.GroupVersionResource, name string, subresource string, err error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 3 {
		return gvr, "", "", ErrInvalidPath
	}

	if parts[0] == "api" {
		if len(parts) >= 3 {
			gvr.Version = parts[1]
			gvr.Resource = parts[2]
			if len(parts) >= 4 {
				name = parts[3]
			}
			if len(parts) >= 5 {
				subresource = parts[4]
			}
		}
	} else if parts[0] == "apis" {
		if len(parts) >= 4 {
			gvr.Group = parts[1]
			gvr.Version = parts[2]
			gvr.Resource = parts[3]
			if len(parts) >= 5 {
				name = parts[4]
			}
			if len(parts) >= 6 {
				subresource = parts[5]
			}
		}
	}

	return gvr, name, subresource, nil
}

// BuildPath constructs an API path from the given components.
func (r *PathResolver) BuildPath(gvr schema.GroupVersionResource, name string, subresource string) string {
	var path string

	if gvr.Group == "" {
		path = "/api/" + gvr.Version + "/" + gvr.Resource
	} else {
		path = "/apis/" + gvr.Group + "/" + gvr.Version + "/" + gvr.Resource
	}

	if name != "" {
		path += "/" + name
	}

	if subresource != "" {
		path += "/" + subresource
	}

	return path
}

// ErrInvalidPath is returned when a path cannot be parsed.
var ErrInvalidPath = fmt.Errorf("invalid path")

// APIRoute represents a parsed API route.
type APIRoute struct {
	Method       string
	Path         string
	Verb         string
	Resource     string
	SubResource  string
	Namespace    string
	Name         string
	GroupVersion schema.GroupVersion
}

// ParseAPIRoute parses an API path into an APIRoute structure.
func ParseAPIRoute(path string) (*APIRoute, error) {
	route := &APIRoute{}

	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return nil, ErrInvalidPath
	}

	if parts[0] == "api" {
		if len(parts) >= 2 {
			route.GroupVersion = schema.GroupVersion{Group: "", Version: parts[1]}
		}
		if len(parts) >= 3 {
			route.Resource = parts[2]
		}
		if len(parts) >= 4 {
			route.Name = parts[3]
		}
		if len(parts) >= 5 {
			route.SubResource = parts[4]
		}
	} else if parts[0] == "apis" {
		if len(parts) >= 3 {
			route.GroupVersion = schema.GroupVersion{Group: parts[1], Version: parts[2]}
		}
		if len(parts) >= 4 {
			route.Resource = parts[3]
		}
		if len(parts) >= 5 {
			route.Name = parts[4]
		}
		if len(parts) >= 6 {
			route.SubResource = parts[5]
		}
	} else {
		return nil, ErrInvalidPath
	}

	return route, nil
}
