# CLAUDE.md

本文件指导 Claude Code 在本仓库中工作。这是 **RuoYi-Vue-Plus (Java/Spring Boot) 的 Go (Gin) 重写版**。

## 项目定位

- 参照 `E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus` 的功能，用 Go + Gin 重新实现。
- 架构： **多模块拆进程 + nginx 负载均衡**。每个业务模块编译成独立 binary、独立进程启动。
- 数据库： **MySQL + GORM**，所有进程共用同一个库。Redis 做会话/缓存/分布式锁。

## 目录结构与分层

标准 Go 布局，`internal` 按模块组织，严格三层：

```
cmd/<module>/main.go     进程入口，一个目录 = 一个进程
internal/<module>/
  router.go              路由注册 RegisterRoutes(r, deps)
  handler/               HTTP 层：绑定参数、调 service、返回 response.R
  service/               业务逻辑；system 的 service 导出供 auth 复用
  repository/            数据访问，只有这一层碰 GORM/DB
  model/                 entity(表) / dto(入参) / vo(出参)
pkg/                     可复用公共库(response/config/database/redis/middleware/auth/constant)
configs/                 application.yaml(公共) + <module>.yaml(各进程端口)
deploy/                  nginx.conf / Dockerfile / docker-compose.yaml
```

**分层依赖是单向的**：`handler → service → repository → model`。禁止反向依赖，禁止 handler 直连 repository。

## 关键约定（务必遵守）

1. **统一响应**：handler 一律返回 `pkg/response` 的 `R[T]` / `PageResult[T]`，用 `response.Ok/Fail/FailCode` 构造。不要手写
   `gin.H` 拼响应。
2. **auth 复用 system 走 in-process**：`internal/auth/service` 直接 `import internal/system/service`，同进程函数调用， **不走
   HTTP**。auth 进程也连同一数据库。
3. **只有 repository 层接触 GORM**。service 拿到的是 model，不感知 SQL。
4. **模块隔离**：`internal/system` 与 `internal/auth` 之间只能通过 service 层导出的接口交互，不要跨模块 import
   handler/repository/model 内部。
5. **命名**：包名小写无下划线；文件名 `xxx_handler.go`/`xxx_service.go`/`xxx_repository.go`；实体对应表名（`SysUser` → 表
   `sys_user`）。
6. **错误处理**：service 返回 `error`，由 handler 统一转成 `response.Fail`；全局异常兜底在 `pkg/middleware`。

## 常用命令

```bash
go build ./...              # 全量编译（提交前必过）
go vet ./...                # 静态检查（提交前必过）
go run ./cmd/modular/system  # 启动 system 进程 :9201
go run ./cmd/modular/auth    # 启动 auth 进程   :9210
go run ./cmd/standalone      # 单体进程 auth+system :8080
go mod tidy                 # 引入新依赖后整理

docker compose -f deploy/docker-compose.yaml up --build   # 一键起全套(经 nginx)
```

新增业务代码引入 GORM/redis/viper 等依赖后，务必 `go mod tidy` 并确认 `go build ./...` 通过。

## 从 Java 原项目迁移的对应关系

| RuoYi (Java)                   | 本项目 (Go)                              |
|--------------------------------|------------------------------------------|
| ruoyi-admin(聚合启动)          | cmd/<module>/main.go(每模块一进程)       |
| Controller                     | internal/<module>/handler                |
| IXxxService / XxxServiceImpl   | internal/<module>/service                |
| XxxMapper (MyBatis)            | internal/<module>/repository (GORM)      |
| domain / domain.bo / domain.vo | model(entity / dto / vo)                 |
| ruoyi-common-*                 | pkg/*                                    |
| R<T> / TableDataInfo           | response.R[T] / repository.PageResult[T] |
| PageQuery                      | repository.PageQuery                     |

## 迁移中最需要注意的两个难点

1. **数据权限 `@DataPermission`**：Java 用 SpEL 注解 + MyBatis 拦截器动态拼 SQL 做部门/角色数据隔离。 Go 无等价机制，需在
   repository 层用 **GORM Scopes / Callback** 手写数据权限过滤。这是移植工作量最大处。
2. **sa-token 认证鉴权**：Java 用 sa-token 管登录态、`@SaCheckPermission` 校验权限。 Go 用 **JWT + Redis 存会话** 复刻，权限校验改为
   `pkg/middleware` 里的鉴权中间件（查权限码）。

## 参考

- 原项目：`E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus`（Java 源码，迁移时对照它的业务逻辑）
- 建表 SQL：原项目 `script/sql/` 目录，表结构直接复用
