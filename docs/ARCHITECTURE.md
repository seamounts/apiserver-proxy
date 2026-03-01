# Container Server 架构设计文档

## 1. 项目概述

### 1.1 项目目标

Container Server 是一个轻量级的容器服务 API 网关，参考 kube-apiserver 的设计模式，提供以下核心功能：

- **API 注册机制**：支持注册 K8s 原生资源（Pod、Deployment 等）和自定义资源（CRD）
- **client-go 兼容**：API 接口完全兼容 client-go 调用
- **中间件系统**：支持审计、监控、日志等可插拔中间件
- **智能代理**：自定义 handler 处理注册资源，未注册资源代理到 kube-apiserver
- **数据持久化**：资源对象可存储到数据库

### 1.2 设计原则

1. **简化设计**：相比 kube-apiserver 移除认证、授权、准入控制等复杂功能
2. **可扩展性**：通过中间件和 Hook 机制支持功能扩展
3. **兼容性**：保持与 K8s API 语义兼容
4. **模块化**：API 资源按 Group/Version 分层组织，每个资源独立文件

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           client-go / kubectl                            │
└─────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Container Server                                │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      Middleware Chain                             │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                        │   │
│  │  │  Audit   │→ │ Metrics  │→ │ Logging  │                        │   │
│  │  └──────────┘  └──────────┘  └──────────┘                        │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                      │                                   │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                        Router (go-restful)                        │   │
│  │    /api/v1/{resource}          /apis/{group}/{version}/{resource} │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                      │                                   │
│  ┌───────────────────────┐    ┌───────────────────────┐                │
│  │   Registered Resource │    │  Unregistered Resource │                │
│  │      (Custom Handler) │    │     (Proxy Handler)    │                │
│  └───────────────────────┘    └───────────────────────┘                │
│             │                              │                            │
│             ▼                              ▼                            │
│  ┌───────────────────────┐    ┌───────────────────────┐                │
│  │     Storage Layer     │    │    kube-apiserver     │                │
│  │   (DBStorage/Custom)  │    │      (Proxy)          │                │
│  └───────────────────────┘    └───────────────────────┘                │
│             │                                                         │
│             ▼                                                         │
│  ┌───────────────────────┐                                            │
│  │       Database        │                                            │
│  │     (MySQL/GORM)      │                                            │
│  └───────────────────────┘                                            │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
d:\Workspace\code\apiserver\
├── cmd/
│   └── server/
│       └── main.go              # 主程序入口
├── pkg/
│   ├── api/                     # API 资源注册层
│   │   ├── api.go              # APIManager 统一注册入口
│   │   ├── types/
│   │   │   └── types.go        # APIResource 通用结构体
│   │   ├── corev1/             # Core API Group
│   │   │   ├── register.go     # CoreV1Group 注册入口
│   │   │   ├── pod.go          # Pod 资源存储实现
│   │   │   ├── service.go      # Service 资源存储实现
│   │   │   ├── configmap.go    # ConfigMap 资源存储实现
│   │   │   ├── secret.go       # Secret 资源存储实现
│   │   │   ├── namespace.go    # Namespace 资源存储实现
│   │   │   └── node.go         # Node 资源存储实现
│   │   ├── appsv1/             # Apps API Group
│   │   │   ├── register.go     # AppsV1Group 注册入口
│   │   │   ├── deployment.go   # Deployment 资源存储实现
│   │   │   └── replicaset.go   # ReplicaSet 资源存储实现
│   │   └── batchv1/            # Batch API Group
│   │       ├── register.go     # BatchV1Group 注册入口
│   │       └── job.go          # Job 资源存储实现
│   ├── registry/               # 资源注册核心
│   │   ├── storage.go          # Storage 接口定义
│   │   └── registry.go         # ResourceRegistry, ResourceBuilder
│   ├── server/                 # 核心服务层
│   │   ├── types.go            # 类型定义
│   │   ├── server.go           # 服务初始化和运行
│   │   ├── handler.go          # REST 请求处理器
│   │   └── proxy.go            # 代理到 kube-apiserver
│   ├── storage/                # 存储层
│   │   └── db_storage.go       # GORM 数据库实现
│   ├── middleware/             # 中间件层
│   │   ├── middleware.go       # 中间件接口
│   │   ├── audit.go            # 审计中间件
│   │   ├── metrics.go          # Prometheus 监控
│   │   └── logging.go          # 日志中间件
│   └── router/                 # 路由层
│       └── router.go           # 路由解析
├── docs/
│   └── ARCHITECTURE.md         # 架构设计文档
├── go.mod
└── go.sum
```

---

## 3. 核心组件设计

### 3.1 API 资源注册 (pkg/api)

#### 3.1.1 APIResource 结构体

```go
type APIResource struct {
    Group           string   // API Group (e.g., "apps", "batch", "" for core)
    Version         string   // API Version (e.g., "v1")
    Kind            string   // Resource Kind (e.g., "Pod", "Deployment")
    Resource        string   // Resource name, plural (e.g., "pods", "deployments")
    SingularName    string   // Singular resource name (e.g., "pod", "deployment")
    NamespaceScoped bool     // Whether the resource is namespaced
    ShortNames      []string // Short aliases (e.g., "po" for pods)
    Categories      []string // Resource categories (e.g., "all")
    ObjectType      runtime.Object
    ListObjectType  runtime.Object
    StorageWrapper  func(registry.Storage) registry.Storage
}

func (r *APIResource) GVR() schema.GroupVersionResource
func (r *APIResource) GVK() schema.GroupVersionKind
```

#### 3.1.2 APIManager 统一注册入口

```go
type APIManager struct {
    groups []GroupRegistrar
}

func DefaultAPIManager() *APIManager {
    m := NewAPIManager()
    m.RegisterGroup(corev1.NewCoreV1Group())
    m.RegisterGroup(appsv1.NewAppsV1Group())
    m.RegisterGroup(batchv1.NewBatchV1Group())
    return m
}
```

#### 3.1.3 资源注册示例

```go
// pkg/api/corev1/pod.go
var PodResource = types.APIResource{
    Group:           "",
    Version:         "v1",
    Kind:            "Pod",
    Resource:        "pods",
    SingularName:    "pod",
    NamespaceScoped: true,
    ShortNames:      []string{"po"},
    Categories:      []string{"all"},
    ObjectType:      &corev1.Pod{},
    ListObjectType:  &corev1.PodList{},
    StorageWrapper:  NewPodStorage,
}

// pkg/api/corev1/register.go
func (g *CoreV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
    resources := []types.APIResource{
        PodResource,
        ServiceResource,
        ConfigMapResource,
        SecretResource,
        NamespaceResource,
        NodeResource,
    }
    // ...
}
```

### 3.2 存储接口 (pkg/registry)

#### 3.2.1 Storage 接口

```go
type Storage interface {
    New() runtime.Object
    Create(ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error)
    Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error)
    List(ctx context.Context, options *metav1.ListOptions) (runtime.Object, error)
    Update(ctx context.Context, name string, objInfo UpdatedObjectInfo, ...) (runtime.Object, bool, error)
    Delete(ctx context.Context, name string, deleteValidation ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error)
    DeleteCollection(ctx context.Context, ...) (runtime.Object, error)
    Watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error)
}
```

#### 3.2.2 ResourceBuilder

```go
builder := registry.NewResourceBuilder(gvr).
    SingularName("pod").
    NamespaceScoped(true).
    ShortNames("po").
    Categories("all").
    ObjectType(&corev1.Pod{}).
    ListObjectType(&corev1.PodList{}).
    StorageFactory(factory)
```

#### 3.2.3 ResourceRegistry

```go
type ResourceRegistry struct {
    resources map[string]*ResourceInfo
    groups    map[string]*GroupInfo
}

func (r *ResourceRegistry) Register(builder *ResourceBuilder) error
func (r *ResourceRegistry) Get(gvr schema.GroupVersionResource) (*ResourceInfo, bool)
func (r *ResourceRegistry) GetStorage(gvr schema.GroupVersionResource) (Storage, bool)
func (r *ResourceRegistry) ListResources() []*ResourceInfo
func (r *ResourceRegistry) ListGroups() []*GroupInfo
```

### 3.3 服务核心 (pkg/server)

#### 3.3.1 ContainerServer

```go
type ContainerServer struct {
    config          *Config
    scheme          *runtime.Scheme
    proxyTransport  http.RoundTripper
    kubeRESTConfig  *rest.Config
    middlewareChain []Middleware
    storageRegistry map[schema.GroupVersionResource]Storage
    hookRegistry    *HookRegistry
}
```

#### 3.3.2 配置结构

```go
type Config struct {
    EtcdServers      []string
    KubeAPIServerURL string
    KubeConfig       string
    SecurePort       int
    InsecurePort     int
    EnableProfiling  bool
    EnableMetrics    bool
    DBConfig         *DatabaseConfig
    MiddlewareConfig *MiddlewareConfig
}
```

### 3.4 请求处理 (pkg/server/handler.go)

#### 3.4.1 请求处理流程

```
HTTP Request
     │
     ▼
┌─────────────────┐
│  Parse GVR      │  解析 Group/Version/Resource
└─────────────────┘
     │
     ▼
┌─────────────────┐
│ Lookup Storage  │  查找资源存储
└─────────────────┘
     │
     ├─── Found ──────────────────────┐
     │                                │
     ▼                                ▼
┌─────────────────┐          ┌─────────────────┐
│ Execute Hooks   │          │ Proxy Request   │
└─────────────────┘          └─────────────────┘
     │                                │
     ▼                                ▼
┌─────────────────┐          ┌─────────────────┐
│ Storage Method  │          │ kube-apiserver  │
│ (Create/Get/...)│          │                 │
└─────────────────┘          └─────────────────┘
     │
     ▼
┌─────────────────┐
│ Execute Hooks   │
└─────────────────┘
     │
     ▼
HTTP Response
```

#### 3.4.2 路由设计

| 路径模式 | 说明 | 处理方式 |
|---------|------|---------|
| `/api/v1/{resource}` | Core API (pods, services 等) | 自定义 handler 或 proxy |
| `/api/v1/{resource}/{name}` | 具名资源 | 自定义 handler 或 proxy |
| `/apis/{group}/{version}/{resource}` | Group API (apps/v1/deployments) | 自定义 handler 或 proxy |
| `/apis/{group}/{version}/{resource}/{name}` | 具名 Group 资源 | 自定义 handler 或 proxy |

### 3.5 代理机制 (pkg/server/proxy.go)

#### 3.5.1 代理判断逻辑

```go
func (s *ContainerServer) ShouldProxy(gvr schema.GroupVersionResource, verb Verb) bool {
    _, registered := s.storageRegistry[gvr]
    if !registered {
        return true  // 未注册资源代理到 kube-apiserver
    }
    return false
}
```

### 3.6 数据库存储 (pkg/storage/db_storage.go)

#### 3.6.1 数据模型

```go
type ResourceRecord struct {
    ID              uint           `gorm:"primaryKey"`
    Group           string         `gorm:"index:idx_gvr"`
    Version         string         `gorm:"index:idx_gvr"`
    Resource        string         `gorm:"index:idx_gvr"`
    Namespace       string         `gorm:"index:idx_namespace"`
    Name            string         `gorm:"index:idx_name"`
    UID             string         `gorm:"uniqueIndex:idx_uid"`
    RawData         []byte         `gorm:"type:longblob"`
    ResourceVersion string
    Labels          string
    Annotations     string
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       gorm.DeletedAt `gorm:"index"`
}
```

### 3.7 中间件系统 (pkg/middleware)

#### 3.7.1 中间件接口

```go
type Middleware interface {
    Name() string
    Handler(next http.Handler) http.Handler
}
```

#### 3.7.2 内置中间件

| 中间件 | 功能 | 配置项 |
|-------|------|-------|
| AuditMiddleware | 审计日志 | Level, LogPath, MaxAge |
| MetricsMiddleware | Prometheus 指标 | Namespace, Subsystem, Path |
| LoggingMiddleware | 结构化日志 | Level, Format, OutputPath |

### 3.8 Hook 机制

#### 3.8.1 Hook 类型

```go
const (
    PreCreateHook  HookType = "preCreate"
    PostCreateHook HookType = "postCreate"
    PreUpdateHook  HookType = "preUpdate"
    PostUpdateHook HookType = "postUpdate"
    PreDeleteHook  HookType = "preDelete"
    PostDeleteHook HookType = "postDelete"
    PreGetHook     HookType = "preGet"
    PostGetHook    HookType = "postGet"
    PreListHook    HookType = "preList"
    PostListHook   HookType = "postList"
)
```

---

## 4. 已注册资源

### 4.1 Core API (core/v1)

| 资源 | Kind | Short Names | Namespace Scoped |
|-----|------|-------------|-----------------|
| pods | Pod | po | Yes |
| services | Service | svc | Yes |
| configmaps | ConfigMap | cm | Yes |
| secrets | Secret | sec | Yes |
| namespaces | Namespace | ns | No |
| nodes | Node | no | No |

### 4.2 Apps API (apps/v1)

| 资源 | Kind | Short Names | Namespace Scoped |
|-----|------|-------------|-----------------|
| deployments | Deployment | deploy | Yes |
| replicasets | ReplicaSet | rs | Yes |

### 4.3 Batch API (batch/v1)

| 资源 | Kind | Short Names | Namespace Scoped |
|-----|------|-------------|-----------------|
| jobs | Job | - | Yes |

---

## 5. 扩展开发

### 5.1 添加新的 API Group

1. 创建目录 `pkg/api/{group}v{version}/`

2. 创建 `register.go`:
```go
package customv1

type CustomV1Group struct{}

func NewCustomV1Group() *CustomV1Group {
    return &CustomV1Group{}
}

func (g *CustomV1Group) GroupName() string {
    return "custom.example.com"
}

func (g *CustomV1Group) RegisterResources(r *registry.ResourceRegistry, factory registry.StorageFactory) error {
    // 注册资源...
}
```

3. 创建资源文件 `myresource.go`:
```go
var MyResourceResource = types.APIResource{
    Group:           "custom.example.com",
    Version:         "v1",
    Kind:            "MyResource",
    Resource:        "myresources",
    SingularName:    "myresource",
    NamespaceScoped: true,
    ObjectType:      &MyResource{},
    ListObjectType:  &MyResourceList{},
    StorageWrapper:  NewMyResourceStorage,
}
```

4. 在 `pkg/api/api.go` 中注册:
```go
func DefaultAPIManager() *APIManager {
    m := NewAPIManager()
    // ...
    m.RegisterGroup(customv1.NewCustomV1Group())
    return m
}
```

### 5.2 自定义 Storage 实现

```go
type MyResourceStorage struct {
    registry.Storage
}

func NewMyResourceStorage(s registry.Storage) registry.Storage {
    return &MyResourceStorage{Storage: s}
}

func (s *MyResourceStorage) Create(ctx context.Context, obj runtime.Object, ...) (runtime.Object, error) {
    // 自定义创建逻辑
    return s.Storage.Create(ctx, obj, createValidation, options)
}
```

### 5.3 添加自定义中间件

```go
type CustomMiddleware struct{}

func (m *CustomMiddleware) Name() string {
    return "custom"
}

func (m *CustomMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 前置处理
        next.ServeHTTP(w, r)
        // 后置处理
    })
}

srv.AddMiddleware(&CustomMiddleware{})
```

---

## 6. 部署与使用

### 6.1 编译

```bash
go build ./cmd/server
```

### 6.2 启动参数

| 参数 | 说明 | 默认值 |
|-----|------|-------|
| `-kube-apiserver` | kube-apiserver URL | `http://localhost:8080` |
| `-kubeconfig` | kubeconfig 文件路径 | "" |
| `-insecure-port` | HTTP 端口 | 8080 |
| `-secure-port` | HTTPS 端口 | 6443 |
| `-db-dsn` | 数据库连接字符串 | "" |
| `-enable-audit` | 启用审计 | true |
| `-enable-metrics` | 启用监控 | true |
| `-enable-logging` | 启用日志 | true |

### 6.3 启动示例

```bash
# 基础启动
./server.exe -insecure-port=8080

# 连接数据库
./server.exe -insecure-port=8080 -db-dsn="user:pass@tcp(localhost:3306)/container_server?charset=utf8mb4&parseTime=True&loc=Local"

# 连接 kube-apiserver
./server.exe -insecure-port=8080 -kube-apiserver="https://k8s-api:6443"
```

---

## 7. 技术栈

| 组件 | 技术选型 | 说明 |
|-----|---------|------|
| HTTP 框架 | go-restful | K8s 同款 REST 框架 |
| K8s API | client-go, apimachinery | 官方 SDK |
| ORM | GORM | 数据库操作 |
| 数据库 | MySQL | 资源持久化 |
| 监控 | Prometheus | 指标采集 |
| 日志 | zap | 结构化日志 |

---

## 8. 与 kube-apiserver 对比

| 特性 | kube-apiserver | Container Server |
|-----|---------------|-----------------|
| 认证 | ✓ 多种认证方式 | ✗ 不支持 |
| 授权 | ✓ RBAC/ABAC | ✗ 不支持 |
| 准入控制 | ✓ Webhook/内置 | ✗ 不支持 |
| Etcd 存储 | ✓ 原生支持 | ✗ 不支持 |
| Watch | ✓ 支持 | ✗ 数据库不支持 |
| 自定义存储 | ✗ 仅 Etcd | ✓ 可扩展 |
| 中间件 | ✗ 固定流程 | ✓ 可插拔 |
| Hook | ✗ 通过 Admission | ✓ 内置支持 |
| 代理模式 | ✗ Aggregation Layer | ✓ 内置支持 |

---

## 9. 未来规划

1. **Watch 支持**：通过数据库变更流或轮询实现
2. **TLS 支持**：添加 HTTPS 端口支持
3. **认证集成**：可选的 Token 认证
4. **缓存层**：添加内存缓存提升性能
5. **分布式支持**：多实例部署支持
