package server

import (
	"context"
	"fmt"
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

	createOptions := readCreateOptions(req.Request)
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

	getOptions := readGetOptions(req.Request)
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

	updateOptions := readUpdateOptions(req.Request)
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

	deleteOptions := readDeleteOptions(req.Request)
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

	deleteOptions := readDeleteOptions(req.Request)
	listOptions := readListOptions(req.Request)

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

	getOptions := readGetOptions(req.Request)
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

	updateOptions := readUpdateOptions(req.Request)
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

// readCreateOptions reads CreateOptions from the HTTP request.
func readCreateOptions(req *http.Request) *metav1.CreateOptions {
	opts := &metav1.CreateOptions{}
	query := req.URL.Query()

	if dryRun := query["dryRun"]; len(dryRun) > 0 {
		opts.DryRun = dryRun
	}
	if fieldManager := query.Get("fieldManager"); fieldManager != "" {
		opts.FieldManager = fieldManager
	}
	if fieldValidation := query.Get("fieldValidation"); fieldValidation != "" {
		opts.FieldValidation = fieldValidation
	}

	return opts
}

// readUpdateOptions reads UpdateOptions from the HTTP request.
func readUpdateOptions(req *http.Request) *metav1.UpdateOptions {
	opts := &metav1.UpdateOptions{}
	query := req.URL.Query()

	if dryRun := query["dryRun"]; len(dryRun) > 0 {
		opts.DryRun = dryRun
	}
	if fieldManager := query.Get("fieldManager"); fieldManager != "" {
		opts.FieldManager = fieldManager
	}
	if fieldValidation := query.Get("fieldValidation"); fieldValidation != "" {
		opts.FieldValidation = fieldValidation
	}

	return opts
}

// readDeleteOptions reads DeleteOptions from the HTTP request.
func readDeleteOptions(req *http.Request) *metav1.DeleteOptions {
	opts := &metav1.DeleteOptions{}
	query := req.URL.Query()

	if dryRun := query["dryRun"]; len(dryRun) > 0 {
		opts.DryRun = dryRun
	}
	if gracePeriodSeconds := query.Get("gracePeriodSeconds"); gracePeriodSeconds != "" {
		if secs, err := parseInt64(gracePeriodSeconds); err == nil {
			opts.GracePeriodSeconds = &secs
		}
	}
	if propagationPolicy := query.Get("propagationPolicy"); propagationPolicy != "" {
		policy := metav1.DeletionPropagation(propagationPolicy)
		opts.PropagationPolicy = &policy
	}
	if orphanDependents := query.Get("orphanDependents"); orphanDependents != "" {
		if val, err := parseBool(orphanDependents); err == nil {
			opts.OrphanDependents = &val
		}
	}

	return opts
}

// readGetOptions reads GetOptions from the HTTP request.
func readGetOptions(req *http.Request) *metav1.GetOptions {
	opts := &metav1.GetOptions{}
	query := req.URL.Query()

	if resourceVersion := query.Get("resourceVersion"); resourceVersion != "" {
		opts.ResourceVersion = resourceVersion
	}

	return opts
}

// readListOptions reads ListOptions from the HTTP request.
func readListOptions(req *http.Request) *metav1.ListOptions {
	opts := &metav1.ListOptions{}
	query := req.URL.Query()

	if labelSelector := query.Get("labelSelector"); labelSelector != "" {
		opts.LabelSelector = labelSelector
	}
	if fieldSelector := query.Get("fieldSelector"); fieldSelector != "" {
		opts.FieldSelector = fieldSelector
	}
	if watch := query.Get("watch"); watch != "" {
		if val, err := parseBool(watch); err == nil {
			opts.Watch = val
		}
	}
	if allowWatchBookmarks := query.Get("allowWatchBookmarks"); allowWatchBookmarks != "" {
		if val, err := parseBool(allowWatchBookmarks); err == nil {
			opts.AllowWatchBookmarks = val
		}
	}
	if resourceVersion := query.Get("resourceVersion"); resourceVersion != "" {
		opts.ResourceVersion = resourceVersion
	}
	if resourceVersionMatch := query.Get("resourceVersionMatch"); resourceVersionMatch != "" {
		opts.ResourceVersionMatch = metav1.ResourceVersionMatch(resourceVersionMatch)
	}
	if timeoutSeconds := query.Get("timeoutSeconds"); timeoutSeconds != "" {
		if secs, err := parseInt64(timeoutSeconds); err == nil {
			opts.TimeoutSeconds = &secs
		}
	}
	if limit := query.Get("limit"); limit != "" {
		if val, err := parseInt64(limit); err == nil {
			opts.Limit = val
		}
	}
	if continueToken := query.Get("continue"); continueToken != "" {
		opts.Continue = continueToken
	}

	return opts
}

func parseInt64(s string) (int64, error) {
	var result int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid int64: %s", s)
		}
		result = result*10 + int64(c-'0')
	}
	return result, nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool: %s", s)
	}
}
