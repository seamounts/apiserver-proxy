package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful/v3"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// handleResourceList handles LIST requests for resources.
func (s *ContainerServer) handleResourceList(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	ctx := req.Request.Context()
	namespace := req.PathParameter("namespace")

	listOptions := &metav1.ListOptions{}
	if err := req.ReadEntity(listOptions); err != nil {
		listOptions = &metav1.ListOptions{}
	}

	result, err := info.Storage.List(ctx, listOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if namespace != "" {
		if err := s.hookRegistry.ExecutePostListHooks(ctx, info.GVR, result); err != nil {
			resp.WriteError(http.StatusInternalServerError, err)
			return
		}
	}

	resp.WriteEntity(result)
}

// handleResourceCreate handles CREATE requests for resources.
func (s *ContainerServer) handleResourceCreate(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	ctx := req.Request.Context()
	namespace := req.PathParameter("namespace")

	obj := info.Storage.New()
	if err := req.ReadEntity(obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if meta, ok := obj.(metav1.Object); ok && namespace != "" {
		meta.SetNamespace(namespace)
	}

	if err := s.hookRegistry.ExecutePreCreateHooks(ctx, info.GVR, obj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	createOptions := &metav1.CreateOptions{}
	result, err := info.Storage.Create(ctx, obj, nil, createOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostCreateHooks(ctx, info.GVR, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeaderAndEntity(http.StatusCreated, result)
}

// handleResourceGet handles GET requests for resources.
func (s *ContainerServer) handleResourceGet(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	ctx := req.Request.Context()
	name := req.PathParameter("name")

	getOptions := &metav1.GetOptions{}
	if err := req.ReadEntity(getOptions); err != nil {
		getOptions = &metav1.GetOptions{}
	}

	result, err := info.Storage.Get(ctx, name, getOptions)
	if err != nil {
		resp.WriteError(http.StatusNotFound, err)
		return
	}

	if err := s.hookRegistry.ExecutePostGetHooks(ctx, info.GVR, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

// handleResourceUpdate handles UPDATE requests for resources.
func (s *ContainerServer) handleResourceUpdate(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	ctx := req.Request.Context()
	name := req.PathParameter("name")
	namespace := req.PathParameter("namespace")

	obj := info.Storage.New()
	if err := req.ReadEntity(obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if meta, ok := obj.(metav1.Object); ok && namespace != "" {
		meta.SetNamespace(namespace)
	}

	if err := s.hookRegistry.ExecutePreUpdateHooks(ctx, info.GVR, obj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	updateOptions := &metav1.UpdateOptions{}
	result, _, err := info.Storage.Update(ctx, name, &simpleUpdatedObjectInfo{obj: obj}, nil, nil, false, updateOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostUpdateHooks(ctx, info.GVR, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

// handleResourcePatch handles PATCH requests for resources.
func (s *ContainerServer) handleResourcePatch(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	s.proxyRequest(req, resp)
}

// handleResourceDelete handles DELETE requests for resources.
func (s *ContainerServer) handleResourceDelete(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	ctx := req.Request.Context()
	name := req.PathParameter("name")

	result, err := info.Storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		resp.WriteError(http.StatusNotFound, err)
		return
	}

	if err := s.hookRegistry.ExecutePreDeleteHooks(ctx, info.GVR, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	deleteOptions := &metav1.DeleteOptions{}
	deletedObj, _, err := info.Storage.Delete(ctx, name, nil, deleteOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostDeleteHooks(ctx, info.GVR, deletedObj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(deletedObj)
}

// handleResourceDeleteCollection handles DELETECOLLECTION requests for resources.
func (s *ContainerServer) handleResourceDeleteCollection(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	ctx := req.Request.Context()

	deleteOptions := &metav1.DeleteOptions{}
	listOptions := &metav1.ListOptions{}

	result, err := info.Storage.DeleteCollection(ctx, nil, deleteOptions, listOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

// handleResourceWatch handles WATCH requests for resources.
func (s *ContainerServer) handleResourceWatch(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo) {
	s.proxyRequest(req, resp)
}

// handleSubresourceGet handles GET requests for subresources.
func (s *ContainerServer) handleSubresourceGet(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo, sr *registry.SubresourceInfo) {
	ctx := req.Request.Context()
	name := req.PathParameter("name")

	getOptions := &metav1.GetOptions{}
	result, err := sr.Storage.Get(ctx, name, getOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

// handleSubresourceUpdate handles UPDATE requests for subresources.
func (s *ContainerServer) handleSubresourceUpdate(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo, sr *registry.SubresourceInfo) {
	ctx := req.Request.Context()
	name := req.PathParameter("name")

	obj := sr.Storage.New()
	if err := req.ReadEntity(obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	updateOptions := &metav1.UpdateOptions{}
	result, _, err := sr.Storage.Update(ctx, name, &simpleUpdatedObjectInfo{obj: obj}, nil, nil, updateOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

// handleSubresourcePatch handles PATCH requests for subresources.
func (s *ContainerServer) handleSubresourcePatch(req *restful.Request, resp *restful.Response, info *registry.ResourceInfo, sr *registry.SubresourceInfo) {
	s.proxyRequest(req, resp)
}

type simpleUpdatedObjectInfo struct {
	obj runtime.Object
}

func (i *simpleUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	return i.obj, nil
}

var _ registry.UpdatedObjectInfo = &simpleUpdatedObjectInfo{}

func parseGVRFromPath(path string) schema.GroupVersionResource {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) >= 3 {
		if parts[0] == "api" {
			return schema.GroupVersionResource{
				Group:    "",
				Version:  parts[1],
				Resource: parts[2],
			}
		} else if parts[0] == "apis" && len(parts) >= 4 {
			return schema.GroupVersionResource{
				Group:    parts[1],
				Version:  parts[2],
				Resource: parts[3],
			}
		}
	}
	return schema.GroupVersionResource{}
}
