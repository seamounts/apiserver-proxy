package router

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Route struct {
	Method      string
	Path        string
	Handler     http.HandlerFunc
	GVR         schema.GroupVersionResource
	Verb        string
	SubResource string
}

type Router struct {
	routes         []*Route
	groups         map[string]*GroupRouter
	parameterCodec ParameterCodec
}

type GroupRouter struct {
	group    string
	versions map[string]*VersionRouter
}

type VersionRouter struct {
	groupVersion schema.GroupVersion
	resources    map[string]*ResourceRouter
}

type ResourceRouter struct {
	gvr        schema.GroupVersionResource
	storage    interface{}
	subrouter  map[string]*SubresourceRouter
}

type SubresourceRouter struct {
	gvr        schema.GroupVersionResource
	subresource string
	storage    interface{}
}

type ParameterCodec interface {
	DecodeParameters(req *http.Request, into interface{}) error
	EncodeParameters(obj interface{}, req *http.Request) error
}

func NewRouter() *Router {
	return &Router{
		routes: make([]*Route, 0),
		groups: make(map[string]*GroupRouter),
	}
}

func (r *Router) AddRoute(route *Route) {
	r.routes = append(r.routes, route)
}

func (r *Router) Routes() []*Route {
	return r.routes
}

func (r *Router) Match(req *http.Request) (*Route, bool) {
	for _, route := range r.routes {
		if route.Method == req.Method && matchPath(route.Path, req.URL.Path) {
			return route, true
		}
	}
	return nil, false
}

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

func (r *Router) Group(group string) *GroupRouter {
	if _, exists := r.groups[group]; !exists {
		r.groups[group] = &GroupRouter{
			group:    group,
			versions: make(map[string]*VersionRouter),
		}
	}
	return r.groups[group]
}

func (g *GroupRouter) Version(version string) *VersionRouter {
	if _, exists := g.versions[version]; !exists {
		g.versions[version] = &VersionRouter{
			groupVersion: schema.GroupVersion{Group: g.group, Version: version},
			resources:    make(map[string]*ResourceRouter),
		}
	}
	return g.versions[version]
}

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

type PathResolver struct {
	groupPrefix string
}

func NewPathResolver(groupPrefix string) *PathResolver {
	return &PathResolver{
		groupPrefix: groupPrefix,
	}
}

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

var ErrInvalidPath = fmt.Errorf("invalid path")

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
