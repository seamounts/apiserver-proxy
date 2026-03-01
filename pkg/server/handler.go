package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful/v3"
	"github.com/seamounts/apiserver-proxy/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type RESTHandler struct {
	server     *ContainerServer
	gvr        schema.GroupVersionResource
	storage    registry.Storage
	proxy      bool
	proxyVerbs map[Verb]bool
}

func NewRESTHandler(server *ContainerServer, gvr schema.GroupVersionResource, storage registry.Storage, proxy bool, proxyVerbs []Verb) *RESTHandler {
	proxyVerbMap := make(map[Verb]bool)
	for _, v := range proxyVerbs {
		proxyVerbMap[v] = true
	}
	return &RESTHandler{
		server:     server,
		gvr:        gvr,
		storage:    storage,
		proxy:      proxy,
		proxyVerbs: proxyVerbMap,
	}
}

func (h *RESTHandler) shouldProxy(verb Verb) bool {
	return h.proxy && h.proxyVerbs[verb]
}

func (s *ContainerServer) handleCreate(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	group := req.PathParameter("group")
	version := req.PathParameter("version")
	resource := req.PathParameter("resource")

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	handler := NewRESTHandler(s, gvr, storage, true, []Verb{})
	if handler.shouldProxy(VerbCreate) {
		s.proxyRequest(req, resp)
		return
	}

	body, err := io.ReadAll(req.Request.Body)
	if err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	obj := storage.New()
	if err := json.Unmarshal(body, obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if err := s.hookRegistry.ExecutePreCreateHooks(ctx, gvr, obj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	createOptions := &metav1.CreateOptions{}
	if err := req.ReadEntity(createOptions); err != nil {
		createOptions = &metav1.CreateOptions{}
	}

	result, err := storage.Create(ctx, obj, nil, createOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostCreateHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeaderAndEntity(http.StatusCreated, result)
}

func (s *ContainerServer) handleGet(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	group := req.PathParameter("group")
	version := req.PathParameter("version")
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	handler := NewRESTHandler(s, gvr, storage, true, []Verb{})
	if handler.shouldProxy(VerbGet) {
		s.proxyRequest(req, resp)
		return
	}

	getOptions := &metav1.GetOptions{}
	if err := req.ReadEntity(getOptions); err != nil {
		getOptions = &metav1.GetOptions{}
	}

	result, err := storage.Get(ctx, name, getOptions)
	if err != nil {
		resp.WriteError(http.StatusNotFound, err)
		return
	}

	if err := s.hookRegistry.ExecutePostGetHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

func (s *ContainerServer) handleList(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	group := req.PathParameter("group")
	version := req.PathParameter("version")
	resource := req.PathParameter("resource")

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	handler := NewRESTHandler(s, gvr, storage, true, []Verb{})
	if handler.shouldProxy(VerbList) {
		s.proxyRequest(req, resp)
		return
	}

	listOptions := &metav1.ListOptions{}
	if err := req.ReadEntity(listOptions); err != nil {
		listOptions = &metav1.ListOptions{}
	}

	result, err := storage.List(ctx, listOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostListHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

func (s *ContainerServer) handleUpdate(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	group := req.PathParameter("group")
	version := req.PathParameter("version")
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	handler := NewRESTHandler(s, gvr, storage, true, []Verb{})
	if handler.shouldProxy(VerbUpdate) {
		s.proxyRequest(req, resp)
		return
	}

	body, err := io.ReadAll(req.Request.Body)
	if err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	obj := storage.New()
	if err := json.Unmarshal(body, obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if err := s.hookRegistry.ExecutePreUpdateHooks(ctx, gvr, obj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	updateOptions := &metav1.UpdateOptions{}
	if err := req.ReadEntity(updateOptions); err != nil {
		updateOptions = &metav1.UpdateOptions{}
	}

	result, _, err := storage.Update(ctx, name, &simpleUpdatedObjectInfo{obj: obj}, nil, nil, false, updateOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostUpdateHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

func (s *ContainerServer) handlePatch(req *restful.Request, resp *restful.Response) {
	s.proxyRequest(req, resp)
}

func (s *ContainerServer) handleDelete(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	group := req.PathParameter("group")
	version := req.PathParameter("version")
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	handler := NewRESTHandler(s, gvr, storage, true, []Verb{})
	if handler.shouldProxy(VerbDelete) {
		s.proxyRequest(req, resp)
		return
	}

	deleteOptions := &metav1.DeleteOptions{}
	if err := req.ReadEntity(deleteOptions); err != nil {
		deleteOptions = &metav1.DeleteOptions{}
	}

	result, err := storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		resp.WriteError(http.StatusNotFound, err)
		return
	}

	if err := s.hookRegistry.ExecutePreDeleteHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	deletedObj, _, err := storage.Delete(ctx, name, nil, deleteOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostDeleteHooks(ctx, gvr, deletedObj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(deletedObj)
}

// handleSubresource handles subresource requests (e.g., pods/status, deployments/scale).
func (s *ContainerServer) handleSubresource(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	group := req.PathParameter("group")
	version := req.PathParameter("version")
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")
	subresource := req.PathParameter("subresource")

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	srInfo, exists := s.resourceRegistry.GetSubresource(gvr, subresource)
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	switch req.Request.Method {
	case http.MethodGet:
		s.handleSubresourceGet(ctx, req, resp, srInfo, name)
	case http.MethodPut, http.MethodPatch:
		s.handleSubresourceUpdate(ctx, req, resp, srInfo, name)
	default:
		resp.WriteError(http.StatusMethodNotAllowed, fmt.Errorf("method %s not allowed for subresource", req.Request.Method))
	}
}

// handleSubresourceGet handles GET requests for subresources.
func (s *ContainerServer) handleSubresourceGet(ctx context.Context, req *restful.Request, resp *restful.Response, srInfo *registry.SubresourceInfo, name string) {
	getOptions := &metav1.GetOptions{}
	if err := req.ReadEntity(getOptions); err != nil {
		getOptions = &metav1.GetOptions{}
	}

	result, err := srInfo.Storage.Get(ctx, name, getOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

// handleSubresourceUpdate handles PUT/PATCH requests for subresources.
func (s *ContainerServer) handleSubresourceUpdate(ctx context.Context, req *restful.Request, resp *restful.Response, srInfo *registry.SubresourceInfo, name string) {
	body, err := io.ReadAll(req.Request.Body)
	if err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	obj := srInfo.Storage.New()
	if err := json.Unmarshal(body, obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	updateOptions := &metav1.UpdateOptions{}
	if err := req.ReadEntity(updateOptions); err != nil {
		updateOptions = &metav1.UpdateOptions{}
	}

	result, _, err := srInfo.Storage.Update(ctx, name, &simpleUpdatedObjectInfo{obj: obj}, nil, nil, updateOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

func (s *ContainerServer) handleCoreCreate(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	resource := req.PathParameter("resource")

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	body, err := io.ReadAll(req.Request.Body)
	if err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	obj := storage.New()
	if err := json.Unmarshal(body, obj); err != nil {
		resp.WriteError(http.StatusBadRequest, err)
		return
	}

	if err := s.hookRegistry.ExecutePreCreateHooks(ctx, gvr, obj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	createOptions := &metav1.CreateOptions{}
	result, err := storage.Create(ctx, obj, nil, createOptions)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostCreateHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeaderAndEntity(http.StatusCreated, result)
}

func (s *ContainerServer) handleCoreGet(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	result, err := storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		resp.WriteError(http.StatusNotFound, err)
		return
	}

	if err := s.hookRegistry.ExecutePostGetHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

func (s *ContainerServer) handleCoreList(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	resource := req.PathParameter("resource")

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	result, err := storage.List(ctx, &metav1.ListOptions{})
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostListHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(result)
}

func (s *ContainerServer) handleCoreUpdate(req *restful.Request, resp *restful.Response) {
	s.proxyRequest(req, resp)
}

func (s *ContainerServer) handleCorePatch(req *restful.Request, resp *restful.Response) {
	s.proxyRequest(req, resp)
}

func (s *ContainerServer) handleCoreDelete(req *restful.Request, resp *restful.Response) {
	ctx := req.Request.Context()
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}

	storage, exists := s.storageRegistry[gvr]
	if !exists {
		s.proxyRequest(req, resp)
		return
	}

	result, err := storage.Get(ctx, name, &metav1.GetOptions{})
	if err != nil {
		resp.WriteError(http.StatusNotFound, err)
		return
	}

	if err := s.hookRegistry.ExecutePreDeleteHooks(ctx, gvr, result); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	deletedObj, _, err := storage.Delete(ctx, name, nil, &metav1.DeleteOptions{})
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err := s.hookRegistry.ExecutePostDeleteHooks(ctx, gvr, deletedObj); err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	resp.WriteEntity(deletedObj)
}

type simpleUpdatedObjectInfo struct {
	obj runtime.Object
}

func (i *simpleUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	return i.obj, nil
}

var _ registry.UpdatedObjectInfo = &simpleUpdatedObjectInfo{}

func parseGVRFromPath(path string) schema.GroupVersionResource {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 {
		return schema.GroupVersionResource{
			Group:    parts[0],
			Version:  parts[1],
			Resource: parts[2],
		}
	}
	return schema.GroupVersionResource{}
}
