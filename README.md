# RuoYi-Go-Vue-Plus

RuoYi-Vue-Plus 的 Go (Gin) 实现，采用标准 Go 项目布局 + 多模块拆进程 + nginx 负载均衡。 每个业务模块编译成独立
binary、独立进程启动，nginx 按路径前缀路由并对热点模块做负载均衡。

## 目录结构

```
.
├── cmd/                        # 各进程入口(一个子目录 = 一个进程)
│   ├── auth/main.go            #   认证服务   :8080
│   └── system/main.go          #   系统管理   :8081
│
├── internal/                   # 私有业务代码，按模块组织
│   ├── system/                 #   系统管理(用户/角色/菜单/部门)
│   │   ├── router.go           #     路由注册
│   │   ├── handler/            #     HTTP 层
│   │   ├── service/            #     业务逻辑(导出供 auth 复用)
│   │   ├── repository/         #     数据访问(GORM)
│   │   └── model/              #     entity / dto / vo
│   └── auth/                   #   认证(登录/登出/验证码)
│       ├── router.go
│       ├── handler/
│       ├── service/            #     in-process 复用 system 的 service
│       └── model/
│
├── pkg/                        # 可复用公共库
│   ├── response/               #   统一响应 R / 分页 PageResult
│   ├── config/                 #   配置加载
│   ├── database/               #   MySQL + GORM
│   ├── redis/                  #   Redis 客户端
│   ├── middleware/             #   Gin 中间件(CORS/异常/日志/鉴权)
│   ├── auth/                   #   JWT + Redis 认证鉴权
│   └── constant/               #   全局常量
│
├── configs/                    # 配置文件
│   ├── application.yaml         #   公共(DB/Redis/JWT)
│   ├── auth.yaml / system.yaml  #   各进程端口
│
└── deploy/                     # nginx.conf / Dockerfile / docker-compose.yaml
```

## 架构要点

- **多进程拆模块**：`cmd/<module>/main.go` 一个目录一个进程，哪个模块热点就多起几个实例。
- **auth in-process 复用 system**：`internal/auth/service` 直接 import `internal/system/service`， 同进程函数调用，无网络开销；auth
  进程因此也连接同一数据库。
- **共用一个 MySQL 库**，Redis 做会话/缓存/分布式锁。

## 运行

```bash
go build ./...
go run ./cmd/system    # :8081/system/ping
go run ./cmd/auth      # :8080/auth/ping

# Docker 一键(经 nginx)
docker compose -f deploy/docker-compose.yaml up --build
```

## 移植待办

- **数据权限 (@DataScope)**：用 GORM Scopes/Callback 复刻部门数据隔离。
- **sa-token 会话/权限**：JWT + Redis 复刻登录态，权限校验改为中间件。
