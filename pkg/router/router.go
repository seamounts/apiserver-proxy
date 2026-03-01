// Package router provides HTTP routing functionality for the API server.
// It implements automatic route generation based on registered API resources,
// using go-restful as the underlying REST framework.
package router

import (
	"fmt"
	"net/http"

	"github.com/emicklei/go-restful/v3"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandlerFunc is a function type for handling REST requests.
type HandlerFunc func(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo)

// SubresourceHandlerFunc is a function type for handling subresource requests.
type SubresourceHandlerFunc func(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo, sr *registry.SubresourceInfo)

// Handlers contains the handler functions for resource operations.
type Handlers struct {
	List              HandlerFunc
	Create            HandlerFunc
	Get               HandlerFunc
	Update            HandlerFunc
	Patch             HandlerFunc
	Delete            HandlerFunc
	DeleteCollection  HandlerFunc
	Watch             HandlerFunc
	SubresourceGet    SubresourceHandlerFunc
	SubresourceUpdate SubresourceHandlerFunc
	SubresourcePatch  SubresourceHandlerFunc
}

// APIGroupInstaller installs REST routes for an API group.
type APIGroupInstaller struct {
	groupName    string
	groupVersion schema.GroupVersion
	registry     *registry.ResourceRegistry
	handlers     *Handlers
}

// NewAPIGroupInstaller creates a new APIGroupInstaller.
func NewAPIGroupInstaller(gv schema.GroupVersion, reg *registry.ResourceRegistry, handlers *Handlers) *APIGroupInstaller {
	return &APIGroupInstaller{
		groupName:    gv.Group,
		groupVersion: gv,
		registry:     reg,
		handlers:     handlers,
	}
}

// Install installs REST routes for all resources in the API group.
func (i *APIGroupInstaller) Install() *restful.WebService {
	ws := new(restful.WebService)

	if i.groupName == "" {
		ws.Path(fmt.Sprintf("/api/%s", i.groupVersion.Version))
	} else {
		ws.Path(fmt.Sprintf("/apis/%s/%s", i.groupName, i.groupVersion.Version))
	}
	ws.Consumes(restful.MIME_JSON, "application/yaml")
	ws.Produces(restful.MIME_JSON, "application/yaml")

	resources := i.getResourcesForGroup()

	for _, resourceInfo := range resources {
		i.installResourceRoutes(ws, resourceInfo)
	}

	return ws
}

// getResourcesForGroup returns all resources for this API group.
func (i *APIGroupInstaller) getResourcesForGroup() []*registry.ResourceInfo {
	var result []*registry.ResourceInfo

	for _, info := range i.registry.ListResources() {
		if info.GVR.Group == i.groupName && info.GVR.Version == i.groupVersion.Version {
			result = append(result, info)
		}
	}

	return result
}

// installResourceRoutes installs routes for a single resource.
func (i *APIGroupInstaller) installResourceRoutes(ws *restful.WebService, info *registry.ResourceInfo) {
	resourcePath := info.GVR.Resource

	verbs := make(map[string]bool)
	for _, v := range info.Verbs {
		verbs[v] = true
	}

	if info.NamespaceScoped {
		i.installNamespacedRoutes(ws, resourcePath, info, verbs)
	} else {
		i.installClusterScopedRoutes(ws, resourcePath, info, verbs)
	}

	for _, sr := range info.Subresources {
		i.installSubresourceRoutes(ws, resourcePath, info, sr)
	}
}

// installNamespacedRoutes installs routes for namespace-scoped resources.
func (i *APIGroupInstaller) installNamespacedRoutes(ws *restful.WebService, resourcePath string, info *registry.ResourceInfo, verbs map[string]bool) {
	basePath := fmt.Sprintf("/namespaces/{namespace}/%s", resourcePath)
	itemPath := fmt.Sprintf("%s/{name}", basePath)

	if verbs["list"] {
		ws.Route(ws.GET(basePath).
			To(i.createListHandler(info)).
			Doc(fmt.Sprintf("List %s in a namespace", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Writes(info.ListObjectType))
	}

	if verbs["create"] {
		ws.Route(ws.POST(basePath).
			To(i.createCreateHandler(info)).
			Doc(fmt.Sprintf("Create a %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Reads(info.ObjectType).
			Writes(info.ObjectType))
	}

	if verbs["deletecollection"] {
		ws.Route(ws.DELETE(basePath).
			To(i.createDeleteCollectionHandler(info)).
			Doc(fmt.Sprintf("Delete collection of %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")))
	}

	if verbs["get"] {
		ws.Route(ws.GET(itemPath).
			To(i.createGetHandler(info)).
			Doc(fmt.Sprintf("Get a %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Param(ws.PathParameter("name", "resource name")).
			Writes(info.ObjectType))
	}

	if verbs["update"] {
		ws.Route(ws.PUT(itemPath).
			To(i.createUpdateHandler(info)).
			Doc(fmt.Sprintf("Update a %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Param(ws.PathParameter("name", "resource name")).
			Reads(info.ObjectType).
			Writes(info.ObjectType))
	}

	if verbs["patch"] {
		ws.Route(ws.PATCH(itemPath).
			To(i.createPatchHandler(info)).
			Doc(fmt.Sprintf("Patch a %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Param(ws.PathParameter("name", "resource name")).
			Writes(info.ObjectType))
	}

	if verbs["delete"] {
		ws.Route(ws.DELETE(itemPath).
			To(i.createDeleteHandler(info)).
			Doc(fmt.Sprintf("Delete a %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Param(ws.PathParameter("name", "resource name")).
			Writes(info.ObjectType))
	}

	if verbs["watch"] {
		ws.Route(ws.GET(basePath).
			To(i.createWatchHandler(info)).
			Doc(fmt.Sprintf("Watch %s", info.SingularName)).
			Param(ws.PathParameter("namespace", "namespace name")).
			Param(ws.QueryParameter("watch", "watch the resource")))
	}
}

// installClusterScopedRoutes installs routes for cluster-scoped resources.
func (i *APIGroupInstaller) installClusterScopedRoutes(ws *restful.WebService, resourcePath string, info *registry.ResourceInfo, verbs map[string]bool) {
	basePath := fmt.Sprintf("/%s", resourcePath)
	itemPath := fmt.Sprintf("%s/{name}", basePath)

	if verbs["list"] {
		ws.Route(ws.GET(basePath).
			To(i.createListHandler(info)).
			Doc(fmt.Sprintf("List all %s", info.SingularName)).
			Writes(info.ListObjectType))
	}

	if verbs["create"] {
		ws.Route(ws.POST(basePath).
			To(i.createCreateHandler(info)).
			Doc(fmt.Sprintf("Create a %s", info.SingularName)).
			Reads(info.ObjectType).
			Writes(info.ObjectType))
	}

	if verbs["deletecollection"] {
		ws.Route(ws.DELETE(basePath).
			To(i.createDeleteCollectionHandler(info)).
			Doc(fmt.Sprintf("Delete collection of %s", info.SingularName)))
	}

	if verbs["get"] {
		ws.Route(ws.GET(itemPath).
			To(i.createGetHandler(info)).
			Doc(fmt.Sprintf("Get a %s", info.SingularName)).
			Param(ws.PathParameter("name", "resource name")).
			Writes(info.ObjectType))
	}

	if verbs["update"] {
		ws.Route(ws.PUT(itemPath).
			To(i.createUpdateHandler(info)).
			Doc(fmt.Sprintf("Update a %s", info.SingularName)).
			Param(ws.PathParameter("name", "resource name")).
			Reads(info.ObjectType).
			Writes(info.ObjectType))
	}

	if verbs["patch"] {
		ws.Route(ws.PATCH(itemPath).
			To(i.createPatchHandler(info)).
			Doc(fmt.Sprintf("Patch a %s", info.SingularName)).
			Param(ws.PathParameter("name", "resource name")).
			Writes(info.ObjectType))
	}

	if verbs["delete"] {
		ws.Route(ws.DELETE(itemPath).
			To(i.createDeleteHandler(info)).
			Doc(fmt.Sprintf("Delete a %s", info.SingularName)).
			Param(ws.PathParameter("name", "resource name")).
			Writes(info.ObjectType))
	}
}

// installSubresourceRoutes installs routes for subresources.
func (i *APIGroupInstaller) installSubresourceRoutes(ws *restful.WebService, resourcePath string, info *registry.ResourceInfo, sr *registry.SubresourceInfo) {
	var itemPath string

	if info.NamespaceScoped {
		itemPath = fmt.Sprintf("/namespaces/{namespace}/%s/{name}/%s", resourcePath, sr.Name)
	} else {
		itemPath = fmt.Sprintf("/%s/{name}/%s", resourcePath, sr.Name)
	}

	srVerbs := make(map[string]bool)
	for _, v := range sr.Verbs {
		srVerbs[v] = true
	}

	if srVerbs["get"] {
		ws.Route(ws.GET(itemPath).
			To(i.createSubresourceGetHandler(info, sr)).
			Doc(fmt.Sprintf("Get %s/%s", info.SingularName, sr.Name)).
			Writes(info.ObjectType))
	}

	if srVerbs["update"] {
		ws.Route(ws.PUT(itemPath).
			To(i.createSubresourceUpdateHandler(info, sr)).
			Doc(fmt.Sprintf("Update %s/%s", info.SingularName, sr.Name)).
			Reads(info.ObjectType).
			Writes(info.ObjectType))
	}

	if srVerbs["patch"] {
		ws.Route(ws.PATCH(itemPath).
			To(i.createSubresourcePatchHandler(info, sr)).
			Doc(fmt.Sprintf("Patch %s/%s", info.SingularName, sr.Name)).
			Writes(info.ObjectType))
	}
}

// Handler factory methods

func (i *APIGroupInstaller) createListHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.List != nil {
			i.handlers.List(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("list not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createCreateHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.Create != nil {
			i.handlers.Create(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("create not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createGetHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.Get != nil {
			i.handlers.Get(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("get not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createUpdateHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.Update != nil {
			i.handlers.Update(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("update not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createPatchHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.Patch != nil {
			i.handlers.Patch(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("patch not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createDeleteHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.Delete != nil {
			i.handlers.Delete(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("delete not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createDeleteCollectionHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.DeleteCollection != nil {
			i.handlers.DeleteCollection(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("deletecollection not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createWatchHandler(info *registry.ResourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.Watch != nil {
			i.handlers.Watch(req, resp, info)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("watch not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createSubresourceGetHandler(info *registry.ResourceInfo, sr *registry.SubresourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.SubresourceGet != nil {
			i.handlers.SubresourceGet(req, resp, info, sr)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("subresource get not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createSubresourceUpdateHandler(info *registry.ResourceInfo, sr *registry.SubresourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.SubresourceUpdate != nil {
			i.handlers.SubresourceUpdate(req, resp, info, sr)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("subresource update not implemented"))
		}
	}
}

func (i *APIGroupInstaller) createSubresourcePatchHandler(info *registry.ResourceInfo, sr *registry.SubresourceInfo) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if i.handlers.SubresourcePatch != nil {
			i.handlers.SubresourcePatch(req, resp, info, sr)
		} else {
			resp.WriteError(http.StatusNotImplemented, fmt.Errorf("subresource patch not implemented"))
		}
	}
}

// Router manages all API routes.
type Router struct {
	registry *registry.ResourceRegistry
	handlers *Handlers
}

// NewRouter creates a new Router.
func NewRouter(reg *registry.ResourceRegistry, handlers *Handlers) *Router {
	return &Router{
		registry: reg,
		handlers: handlers,
	}
}

// InstallAll installs routes for all registered API groups.
func (r *Router) InstallAll() []*restful.WebService {
	var webServices []*restful.WebService

	groups := r.registry.ListGroups()
	installedGroups := make(map[string]bool)

	for _, group := range groups {
		groupKey := group.GroupVersion.String()
		if installedGroups[groupKey] {
			continue
		}
		installedGroups[groupKey] = true

		installer := NewAPIGroupInstaller(group.GroupVersion, r.registry, r.handlers)
		ws := installer.Install()
		webServices = append(webServices, ws)
	}

	return webServices
}

// InstallAPIGroupsHandler installs the /apis handler.
func InstallAPIGroupsHandler(reg *registry.ResourceRegistry) *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/apis")
	ws.Consumes(restful.MIME_JSON)
	ws.Produces(restful.MIME_JSON)

	ws.Route(ws.GET("/").To(func(req *restful.Request, resp *restful.Response) {
		groups := reg.ListGroups()

		apiGroups := &metav1.APIGroupList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "APIGroupList",
				APIVersion: "v1",
			},
			Groups: make([]metav1.APIGroup, 0, len(groups)),
		}

		for _, group := range groups {
			apiGroup := metav1.APIGroup{
				Name: group.GroupVersion.Group,
				Versions: []metav1.GroupVersionForDiscovery{
					{
						GroupVersion: group.GroupVersion.String(),
						Version:      group.GroupVersion.Version,
					},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: group.GroupVersion.String(),
					Version:      group.GroupVersion.Version,
				},
			}
			apiGroups.Groups = append(apiGroups.Groups, apiGroup)
		}

		resp.WriteEntity(apiGroups)
	}))

	return ws
}
