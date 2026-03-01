package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/emicklei/go-restful/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type ProxyHandler struct {
	config       *Config
	transport    http.RoundTripper
	reverseProxy *httputil.ReverseProxy
	targetURL    *url.URL
}

func NewProxyHandler(cfg *Config, restConfig *rest.Config) (*ProxyHandler, error) {
	targetURL, err := url.Parse(restConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse kube-apiserver URL: %v", err)
	}

	transport, err := rest.TransportFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.Host = targetURL.Host

			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "container-server/1.0")
			}
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(&metav1.Status{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Status",
					APIVersion: "v1",
				},
				Status:  metav1.StatusFailure,
				Code:    http.StatusBadGateway,
				Reason:  metav1.StatusReasonServiceUnavailable,
				Message: fmt.Sprintf("Error proxying request to kube-apiserver: %v", err),
			})
		},
	}

	return &ProxyHandler{
		config:       cfg,
		transport:    transport,
		reverseProxy: proxy,
		targetURL:    targetURL,
	}, nil
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.reverseProxy.ServeHTTP(w, r)
}

type ProxyHookConfig struct {
	EnablePreProxyHooks  bool
	EnablePostProxyHooks bool
}

func (s *ContainerServer) proxyRequest(req *restful.Request, resp *restful.Response) {
	if s.proxyTransport == nil {
		resp.WriteError(http.StatusBadGateway, fmt.Errorf("proxy not configured"))
		return
	}

	httpReq := req.Request
	ctx := httpReq.Context()

	path := httpReq.URL.Path
	if !strings.HasPrefix(path, "/api") && !strings.HasPrefix(path, "/apis") {
		path = "/api" + path
	}

	gvr, verb := parseGVRAndVerbFromPath(path, httpReq.Method)

	if s.hookRegistry != nil {
		switch verb {
		case VerbCreate:
			s.hookRegistry.ExecutePreCreateHooks(ctx, gvr, nil)
		case VerbGet:
			s.hookRegistry.ExecutePreGetHooks(ctx, gvr, nil)
		case VerbList:
			s.hookRegistry.ExecutePreListHooks(ctx, gvr, nil)
		case VerbUpdate:
			s.hookRegistry.ExecutePreUpdateHooks(ctx, gvr, nil)
		case VerbDelete:
			s.hookRegistry.ExecutePreDeleteHooks(ctx, gvr, nil)
		}
	}

	targetURL := s.kubeRESTConfig.Host + path
	if httpReq.URL.RawQuery != "" {
		targetURL += "?" + httpReq.URL.RawQuery
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	proxyReq := &http.Request{
		Method: httpReq.Method,
		URL:    target,
		Header: httpReq.Header.Clone(),
		Body:   httpReq.Body,
	}

	proxyReq = proxyReq.WithContext(ctx)

	proxyResp, err := s.proxyTransport.RoundTrip(proxyReq)
	if err != nil {
		resp.WriteError(http.StatusBadGateway, err)
		return
	}
	defer proxyResp.Body.Close()

	if s.hookRegistry != nil {
		switch verb {
		case VerbCreate:
			s.hookRegistry.ExecutePostCreateHooks(ctx, gvr, nil)
		case VerbGet:
			s.hookRegistry.ExecutePostGetHooks(ctx, gvr, nil)
		case VerbList:
			s.hookRegistry.ExecutePostListHooks(ctx, gvr, nil)
		case VerbUpdate:
			s.hookRegistry.ExecutePostUpdateHooks(ctx, gvr, nil)
		case VerbDelete:
			s.hookRegistry.ExecutePostDeleteHooks(ctx, gvr, nil)
		}
	}

	for k, v := range proxyResp.Header {
		resp.Header()[k] = v
	}

	resp.WriteHeader(proxyResp.StatusCode)

	if proxyResp.Body != nil {
		io.Copy(resp, proxyResp.Body)
	}
}

func (s *ContainerServer) proxyCoreRequest(req *restful.Request, resp *restful.Response) {
	if s.proxyTransport == nil {
		resp.WriteError(http.StatusBadGateway, fmt.Errorf("proxy not configured"))
		return
	}

	httpReq := req.Request
	ctx := httpReq.Context()
	resource := req.PathParameter("resource")
	name := req.PathParameter("name")

	var path string
	if name != "" {
		path = fmt.Sprintf("/api/v1/%s/%s", resource, name)
	} else {
		path = fmt.Sprintf("/api/v1/%s", resource)
	}

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}
	verb := methodToVerb(httpReq.Method, name == "")

	if s.hookRegistry != nil {
		switch verb {
		case VerbCreate:
			s.hookRegistry.ExecutePreCreateHooks(ctx, gvr, nil)
		case VerbGet:
			s.hookRegistry.ExecutePreGetHooks(ctx, gvr, nil)
		case VerbList:
			s.hookRegistry.ExecutePreListHooks(ctx, gvr, nil)
		case VerbUpdate:
			s.hookRegistry.ExecutePreUpdateHooks(ctx, gvr, nil)
		case VerbDelete:
			s.hookRegistry.ExecutePreDeleteHooks(ctx, gvr, nil)
		}
	}

	targetURL := s.kubeRESTConfig.Host + path
	if httpReq.URL.RawQuery != "" {
		targetURL += "?" + httpReq.URL.RawQuery
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		resp.WriteError(http.StatusInternalServerError, err)
		return
	}

	proxyReq := &http.Request{
		Method: httpReq.Method,
		URL:    target,
		Header: httpReq.Header.Clone(),
		Body:   httpReq.Body,
	}

	proxyReq = proxyReq.WithContext(ctx)

	proxyResp, err := s.proxyTransport.RoundTrip(proxyReq)
	if err != nil {
		resp.WriteError(http.StatusBadGateway, err)
		return
	}
	defer proxyResp.Body.Close()

	if s.hookRegistry != nil {
		switch verb {
		case VerbCreate:
			s.hookRegistry.ExecutePostCreateHooks(ctx, gvr, nil)
		case VerbGet:
			s.hookRegistry.ExecutePostGetHooks(ctx, gvr, nil)
		case VerbList:
			s.hookRegistry.ExecutePostListHooks(ctx, gvr, nil)
		case VerbUpdate:
			s.hookRegistry.ExecutePostUpdateHooks(ctx, gvr, nil)
		case VerbDelete:
			s.hookRegistry.ExecutePostDeleteHooks(ctx, gvr, nil)
		}
	}

	for k, v := range proxyResp.Header {
		resp.Header()[k] = v
	}

	resp.WriteHeader(proxyResp.StatusCode)

	if proxyResp.Body != nil {
		io.Copy(resp, proxyResp.Body)
	}
}

type ProxyConfig struct {
	KubeAPIServerURL string
	KubeConfig       string
	ProxyVerbs       []Verb
	ExcludeResources []schema.GroupVersionResource
}

func (s *ContainerServer) ShouldProxy(gvr schema.GroupVersionResource, verb Verb) bool {
	_, registered := s.storageRegistry[gvr]
	if !registered {
		return true
	}

	return false
}

type ProxyInterceptor interface {
	BeforeProxy(ctx context.Context, gvr schema.GroupVersionResource, verb Verb, req *http.Request) error
	AfterProxy(ctx context.Context, gvr schema.GroupVersionResource, verb Verb, resp *http.Response) error
}

type DefaultProxyInterceptor struct{}

func (i *DefaultProxyInterceptor) BeforeProxy(ctx context.Context, gvr schema.GroupVersionResource, verb Verb, req *http.Request) error {
	return nil
}

func (i *DefaultProxyInterceptor) AfterProxy(ctx context.Context, gvr schema.GroupVersionResource, verb Verb, resp *http.Response) error {
	return nil
}

func parseGVRAndVerbFromPath(path string, method string) (schema.GroupVersionResource, Verb) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")

	var gvr schema.GroupVersionResource
	var hasName bool

	if len(parts) >= 3 && parts[0] == "api" {
		gvr.Version = parts[1]
		if len(parts) >= 3 {
			gvr.Resource = parts[2]
		}
		if len(parts) >= 4 {
			hasName = true
		}
	} else if len(parts) >= 4 && parts[0] == "apis" {
		gvr.Group = parts[1]
		gvr.Version = parts[2]
		if len(parts) >= 4 {
			gvr.Resource = parts[3]
		}
		if len(parts) >= 5 {
			hasName = true
		}
	}

	verb := methodToVerb(method, !hasName)
	return gvr, verb
}

func methodToVerb(method string, isList bool) Verb {
	switch method {
	case http.MethodGet:
		if isList {
			return VerbList
		}
		return VerbGet
	case http.MethodPost:
		return VerbCreate
	case http.MethodPut:
		return VerbUpdate
	case http.MethodPatch:
		return VerbPatch
	case http.MethodDelete:
		return VerbDelete
	default:
		return VerbGet
	}
}

type ProxyHookMiddleware struct {
	server *ContainerServer
}

func NewProxyHookMiddleware(server *ContainerServer) *ProxyHookMiddleware {
	return &ProxyHookMiddleware{
		server: server,
	}
}

func (m *ProxyHookMiddleware) Name() string {
	return "proxy-hook"
}

func (m *ProxyHookMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		path := r.URL.Path
		gvr, verb := parseGVRAndVerbFromPath(path, r.Method)

		if m.server.hookRegistry != nil {
			switch verb {
			case VerbCreate:
				m.server.hookRegistry.ExecutePreCreateHooks(ctx, gvr, nil)
			case VerbGet:
				m.server.hookRegistry.ExecutePreGetHooks(ctx, gvr, nil)
			case VerbList:
				m.server.hookRegistry.ExecutePreListHooks(ctx, gvr, nil)
			case VerbUpdate:
				m.server.hookRegistry.ExecutePreUpdateHooks(ctx, gvr, nil)
			case VerbDelete:
				m.server.hookRegistry.ExecutePreDeleteHooks(ctx, gvr, nil)
			}
		}

		rw := &proxyHookResponseWriter{
			ResponseWriter: w,
			server:         m.server,
			gvr:            gvr,
			verb:           verb,
			ctx:            ctx,
		}

		next.ServeHTTP(rw, r)
	})
}

type proxyHookResponseWriter struct {
	http.ResponseWriter
	server      *ContainerServer
	gvr         schema.GroupVersionResource
	verb        Verb
	ctx         context.Context
	wroteHeader bool
}

func (w *proxyHookResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *proxyHookResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.executePostHooks()
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

func (w *proxyHookResponseWriter) executePostHooks() {
	if w.server.hookRegistry != nil {
		switch w.verb {
		case VerbCreate:
			w.server.hookRegistry.ExecutePostCreateHooks(w.ctx, w.gvr, nil)
		case VerbGet:
			w.server.hookRegistry.ExecutePostGetHooks(w.ctx, w.gvr, nil)
		case VerbList:
			w.server.hookRegistry.ExecutePostListHooks(w.ctx, w.gvr, nil)
		case VerbUpdate:
			w.server.hookRegistry.ExecutePostUpdateHooks(w.ctx, w.gvr, nil)
		case VerbDelete:
			w.server.hookRegistry.ExecutePostDeleteHooks(w.ctx, w.gvr, nil)
		}
	}
}

var _ runtime.Object = (*ProxyHookContext)(nil)

type ProxyHookContext struct {
	GVR      schema.GroupVersionResource
	Verb     Verb
	Path     string
	Method   string
	Request  *http.Request
	Response *http.Response
}

func (c *ProxyHookContext) DeepCopyObject() runtime.Object {
	return &ProxyHookContext{
		GVR:    c.GVR,
		Verb:   c.Verb,
		Path:   c.Path,
		Method: c.Method,
	}
}

func (c *ProxyHookContext) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}
