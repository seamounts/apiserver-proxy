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
│   ├── server/                  # 核心服务层
│   │   ├── types.go            # 类型定义
│   │   ├── server.go           # 服务初始化和运行
│   │   ├── handler.go          # REST 请求处理器
│   │   └── proxy.go            # 代理到 kube-apiserver
│   ├── registry/               # 资源注册层
│   │   ├── registry.go         # 存储接口和注册机制
│   │   └── resources.go        # Pod/Deployment 等资源存储
│   ├── storage/                # 存储层
│   │   └── db_storage.go       # GORM 数据库实现
│   ├── middleware/             # 中间件层
│   │   ├── middleware.go       # 中间件接口
│   │   ├── audit.go            # 审计中间件
│   │   ├── metrics.go          # Prometheus 监控
│   │   └── logging.go          # 日志中间件
│   └── router/                 # 路由层
│       └── router.go           # 路由解析
├── go.mod
└── go.sum
```

---

## 3. 核心组件设计

### 3.1 服务核心 (pkg/server)

#### 3.1.1 ContainerServer

```go
type ContainerServer struct {
    config          *Config                              // 服务配置
    scheme          *runtime.Scheme                      // K8s Scheme
    proxyTransport  http.RoundTripper                    // 代理传输层
    kubeRESTConfig  *rest.Config                         // kube-apiserver 配置
    middlewareChain []Middleware                          // 中间件链
    storageRegistry map[schema.GroupVersionResource]Storage  // 资源存储注册表
    hookRegistry    *HookRegistry                         // Hook 注册表
}
```

**核心职责**：
- 初始化服务配置和依赖
- 注册 API 资源和存储
- 管理中间件和 Hook
- 启动 HTTP 服务

#### 3.1.2 配置结构

```go
type Config struct {
    EtcdServers      []string          // Etcd 集群地址（可选）
    KubeAPIServerURL string            // kube-apiserver URL
    KubeConfig       string            // kubeconfig 文件路径
    SecurePort       int               // HTTPS 端口
    InsecurePort     int               // HTTP 端口
    EnableProfiling  bool              // 启用性能分析
    EnableMetrics    bool              // 启用指标
    DBConfig         *DatabaseConfig   // 数据库配置
    MiddlewareConfig *MiddlewareConfig // 中间件配置
}
```

### 3.2 存储接口 (pkg/registry)

#### 3.2.1 Storage 接口

参考 kube-apiserver 的 RESTStorage 设计：

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

#### 3.2.2 资源注册流程

```go
// 1. 创建存储工厂
storageFactory := registry.NewDefaultStorageFactory(func(gvr schema.GroupVersionResource) registry.Storage {
    return storage.NewDBStorage(db, nil, gvr)
})

// 2. 构建资源存储
builder := registry.NewRESTStorageBuilder(schema.GroupVersion{Group: "apps", Version: "v1"})

// 3. 注册资源
registry.RegisterAppsResources(builder, storageFactory)
```

### 3.3 请求处理 (pkg/server/handler.go)

#### 3.3.1 请求处理流程

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

#### 3.3.2 路由设计

| 路径模式 | 说明 | 处理方式 |
|---------|------|---------|
| `/api/v1/{resource}` | Core API (pods, services 等) | 自定义 handler 或 proxy |
| `/api/v1/{resource}/{name}` | 具名资源 | 自定义 handler 或 proxy |
| `/apis/{group}/{version}/{resource}` | Group API (apps/v1/deployments) | 自定义 handler 或 proxy |
| `/apis/{group}/{version}/{resource}/{name}` | 具名 Group 资源 | 自定义 handler 或 proxy |

### 3.4 代理机制 (pkg/server/proxy.go)

#### 3.4.1 代理判断逻辑

```go
func (s *ContainerServer) ShouldProxy(gvr schema.GroupVersionResource, verb Verb) bool {
    _, registered := s.storageRegistry[gvr]
    if !registered {
        return true  // 未注册资源代理到 kube-apiserver
    }
    return false
}
```

#### 3.4.2 代理请求处理

```go
func (s *ContainerServer) proxyRequest(req *restful.Request, resp *restful.Response) {
    // 1. 解析 GVR 和 Verb
    gvr, verb := parseGVRAndVerbFromPath(path, httpReq.Method)
    
    // 2. 执行 Pre Hooks
    s.hookRegistry.ExecutePreCreateHooks(ctx, gvr, nil)
    
    // 3. 构建目标 URL
    targetURL := s.kubeRESTConfig.Host + path
    
    // 4. 创建代理请求
    proxyReq := &http.Request{
        Method: httpReq.Method,
        URL:    target,
        Header: httpReq.Header.Clone(),
        Body:   httpReq.Body,
    }
    
    // 5. 执行代理
    proxyResp, err := s.proxyTransport.RoundTrip(proxyReq)
    
    // 6. 执行 Post Hooks
    s.hookRegistry.ExecutePostCreateHooks(ctx, gvr, nil)
    
    // 7. 转发响应
    resp.WriteHeader(proxyResp.StatusCode)
    io.Copy(resp, proxyResp.Body)
}
```

#### 3.4.3 Proxy Hooks 支持

**方案 1：内置 Proxy Hook 中间件**

```go
// 创建 Proxy Hook 中间件
proxyHookMiddleware := NewProxyHookMiddleware(server)
server.AddMiddleware(proxyHookMiddleware)
```

该中间件会在所有请求（包括 proxy 请求）前后执行 hooks。

**方案 2：在 proxyRequest 方法中直接执行 hooks**

proxy 请求会自动执行对应 verb 的 Pre/Post hooks：

| Verb | Pre Hook | Post Hook |
|------|----------|-----------|
| GET | ExecutePreGetHooks | ExecutePostGetHooks |
| LIST | ExecutePreListHooks | ExecutePostListHooks |
| CREATE | ExecutePreCreateHooks | ExecutePostCreateHooks |
| UPDATE | ExecutePreUpdateHooks | ExecutePostUpdateHooks |
| DELETE | ExecutePreDeleteHooks | ExecutePostDeleteHooks |

**两种方案对比**：

| 特性 | 方案 1 (中间件) | 方案 2 (内置) |
|-----|----------------|--------------|
| 适用范围 | 所有请求 | 仅 proxy 请求 |
| 执行时机 | HTTP 层 | 代理层 |
| 灵活性 | 可插拔 | 始终生效 |
| 性能 | 略低（包装 ResponseWriter） | 略高 |

**推荐用法**：两种方案可以同时使用，中间件层用于全局审计/监控，内置 hooks 用于资源级别的业务逻辑。

### 3.5 数据库存储 (pkg/storage/db_storage.go)

#### 3.5.1 数据模型

```go
type ResourceRecord struct {
    ID              uint           `gorm:"primaryKey"`
    Group           string         `gorm:"index:idx_gvr"`      // API Group
    Version         string         `gorm:"index:idx_gvr"`      // API Version
    Resource        string         `gorm:"index:idx_gvr"`      // Resource Type
    Namespace       string         `gorm:"index:idx_namespace"`// Namespace
    Name            string         `gorm:"index:idx_name"`     // Resource Name
    UID             string         `gorm:"uniqueIndex:idx_uid"`// Unique ID
    RawData         []byte         `gorm:"type:longblob"`      // JSON 序列化数据
    ResourceVersion string                                     // 资源版本
    Labels          string                                     // 标签 JSON
    Annotations     string                                     // 注解 JSON
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       gorm.DeletedAt `gorm:"index"`             // 软删除
}
```

#### 3.5.2 存储操作

| 方法 | 功能 | 数据库操作 |
|-----|------|-----------|
| Create | 创建资源 | INSERT |
| Get | 获取单个资源 | SELECT |
| List | 列表资源 | SELECT (多行) |
| Update | 更新资源 | UPDATE |
| Delete | 删除资源 | DELETE (软删除) |
| DeleteCollection | 批量删除 | DELETE (多行) |

### 3.6 中间件系统 (pkg/middleware)

#### 3.6.1 中间件接口

```go
type Middleware interface {
    Name() string
    Handler(next http.Handler) http.Handler
}
```

#### 3.6.2 内置中间件

| 中间件 | 功能 | 配置项 |
|-------|------|-------|
| AuditMiddleware | 审计日志 | Level, LogPath, MaxAge |
| MetricsMiddleware | Prometheus 指标 | Namespace, Subsystem, Path |
| LoggingMiddleware | 结构化日志 | Level, Format, OutputPath |

#### 3.6.3 中间件链执行

```go
func (c *MiddlewareChain) Handler(final http.Handler) http.Handler {
    for i := len(c.middlewares) - 1; i >= 0; i-- {
        final = c.middlewares[i].Handler(final)
    }
    return final
}
```

### 3.7 Hook 机制

#### 3.7.1 Hook 类型

```go
const (
    PreCreateHook  HookType = "preCreate"   // 创建前
    PostCreateHook HookType = "postCreate"  // 创建后
    PreUpdateHook  HookType = "preUpdate"   // 更新前
    PostUpdateHook HookType = "postUpdate"  // 更新后
    PreDeleteHook  HookType = "preDelete"   // 删除前
    PostDeleteHook HookType = "postDelete"  // 删除后
    PreGetHook     HookType = "preGet"      // 获取前
    PostGetHook    HookType = "postGet"     // 获取后
    PreListHook    HookType = "preList"     // 列表前
    PostListHook   HookType = "postList"    // 列表后
)
```

#### 3.7.2 Hook 注册

```go
srv.AddHook(server.PostCreateHook, func(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
    // 自定义逻辑：如存入数据库、发送事件等
    fmt.Printf("Resource created: %s/%s\n", gvr.Resource, meta.GetName())
    return nil
})
```

---

## 4. API 兼容性设计

### 4.1 client-go 兼容

服务完全兼容 client-go 调用方式：

```go
import (
    "k8s.io/client-go/rest"
    "k8s.io/client-go/kubernetes"
)

// 配置指向 Container Server
config := &rest.Config{
    Host: "http://localhost:8080",
}

// 创建 clientset
clientset, err := kubernetes.NewForConfig(config)

// 正常使用 client-go
pods, err := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
```

### 4.2 API 路径映射

| K8s API 路径 | Container Server 处理 |
|-------------|---------------------|
| `/api/v1/pods` | 本地存储或 proxy |
| `/api/v1/services` | 本地存储或 proxy |
| `/apis/apps/v1/deployments` | 本地存储或 proxy |
| `/apis/apps/v1/statefulsets` | 本地存储或 proxy |
| 其他未注册资源 | 自动 proxy |

---

## 5. 部署与使用

### 5.1 编译

```bash
go build ./cmd/server
```

### 5.2 启动参数

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

### 5.3 启动示例

```bash
# 基础启动
./server.exe -insecure-port=8080

# 连接数据库
./server.exe -insecure-port=8080 -db-dsn="user:pass@tcp(localhost:3306)/container_server?charset=utf8mb4&parseTime=True&loc=Local"

# 连接 kube-apiserver
./server.exe -insecure-port=8080 -kube-apiserver="https://k8s-api:6443"
```

---

## 6. 扩展开发

### 6.1 添加自定义资源

```go
// 1. 定义 GVR
customGVR := schema.GroupVersionResource{
    Group:    "example.com",
    Version:  "v1",
    Resource: "customresources",
}

// 2. 注册资源
registry.RegisterCustomResource(builder, customGVR, storageFactory, "customresource", true)
```

### 6.2 添加自定义中间件

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

// 注册中间件
srv.AddMiddleware(&CustomMiddleware{})
```

### 6.3 添加自定义 Hook

```go
// 资源创建后存入数据库
srv.AddHook(server.PostCreateHook, func(ctx context.Context, gvr schema.GroupVersionResource, obj runtime.Object) error {
    // 保存到自定义存储
    return saveToCustomStorage(obj)
})
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
