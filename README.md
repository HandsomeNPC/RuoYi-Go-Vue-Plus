# RuoYi-Go-Vue-Plus

- **GitHub 地址**：<https://github.com/HandsomeNPC/RuoYi-Go-Vue-Plus>
- **Gitee 地址**：<https://gitee.com/handsome-npc/RuoYi-Go-Vue-Plus>

<p align="center">
  <img src="docs/ruoyi-go-vue-plus-banner.png" alt="RuoYi-Go-Vue-Plus Banner"/>
</p>

> 本仓库基于RuoYi-Go-Vue-Plus 6.X的 Go (Gin) 重写版，致敬原项目作者「疯狂的狮子Li」及若依开源社区。前端项目适配RuoYi-Plus-UI-6.X

## 项目介绍

- **原作者 (Java版 6.X)**：[RuoYi-Vue-Plus](https://gitee.com/dromara/RuoYi-Vue-Plus)（Java 版，dromara / 若依团队）。
- **UI地址 (适配原项目UI 6.X)**: [https://gitee.com/JavaLionLi/plus-ui](https://gitee.com/JavaLionLi/plus-ui)
- **项目文档**: <https://plus-go-doc.chenziwen.top>
- **语言 / 框架**：Go 1.26+ 、Gin、GORM、go-redis。
- **数据库 / 缓存**：MySQL+ Redis
- **主要优势**: golang启动更快 (毫秒级),占用资源更少

## 环境准备

### Go 环境

> 面向不熟悉 Go 的开发者。先配好 Go 工具链，再配代理与工作区，最后验证。

#### 1. 安装 Go

下载并安装 **Go 1.26 及以上**（`go.mod` 锁定 `go 1.26.5`）：

- 官方下载：<https://go.dev/dl/>
- 国内镜像：<https://golang.google.cn/dl/>

Windows 用 msi 安装包，安装时勾选自动配置环境变量；macOS/Linux 用 `tar -C /usr/local -xzf` 解压后手动加 `PATH`。安装完成后验证：

```bash
go version      # 应输出 go1.26.x
go env GOROOT   # Go 安装根目录
```

#### 2. 配置环境变量

Go 用环境变量控制工具链行为，关键字段如下：

| 变量            | 作用                                      | 建议值                               |
|-----------------|-------------------------------------------|--------------------------------------|
| `GOROOT`        | Go 安装目录（含编译器、标准库）           | 安装器默认值，一般不用手改           |
| `GOPATH`        | 工作区：`go install` 装的 bin、缓存的模块 | 默认 `~/go`，不要和项目目录混在一起  |
| `GOPROXY`       | 模块下载代理                              | `https://goproxy.cn,direct`（国内）  |
| `GOSUMDB`       | 校验和数据库                              | `sum.golang.google.cn`（国内对齐上） |
| `GO111MODULE`   | 模块模式开关                              | `on`（1.16+ 默认即 on）              |
| `GOBIN`         | `go install` 产物目录                     | 默认 `$GOPATH/bin`                   |
| `GOOS`/`GOARCH` | 交叉编译目标                              | 本机编译不用设                       |

Windows PowerShell（仅当前会话）：

```powershell
$env:GOPROXY = "https://goproxy.cn,direct"
$env:GO111MODULE = "on"
```

Windows PowerShell（持久，写进用户环境变量，重开终端生效）：

```powershell
[Environment]::SetEnvironmentVariable("GOPROXY", "https://goproxy.cn,direct", "User")
[Environment]::SetEnvironmentVariable("GO111MODULE", "on", "User")
```

Linux / macOS：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GO111MODULE=on
go env -w GOSUMDB=sum.golang.google.cn
```

> `go env -w` 写进 `~/.config/go/env`，对所有项目生效，推荐用这种方式而非改系统环境变量。常用 `GOPROXY` 选项：
> - `https://goproxy.cn,direct` —— 七牛云，国内最稳，优先用这个。
> - `https://goproxy.io,direct` —— 备选。
> - `https://proxy.golang.org,direct` —— 官方代理，国内访问慢。
>   末尾 `,direct` 表示代理拿不到时直连源站（如 GitHub）。

把 `$GOPATH/bin`（默认 `~/go/bin`）加进 `PATH`，这样 `go install` 装的工具（如 `golangci-lint`）能直接用：

```powershell
# Windows
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\go\bin", "User")
```

```bash
# Linux / macOS（写进 ~/.bashrc 或 ~/.zshrc）
export PATH="$PATH:$(go env GOPATH)/bin"
```

#### 3. IDE / 编辑器

推荐 **Visual Studio Code + Go 扩展**，或 **GoLand**（JetBrains）。VS Code 首次打开 `.go` 文件会提示安装 `gopls`
等辅助工具，全部勾选安装即可。

#### 4. 验证

```bash
go env              # 查看所有 Go 环境变量，确认 GOPROXY / GOPATH 生效
go version          # go1.26.x
```

### 数据库与依赖准备

1. **MySQL 8.x**，字符集 `utf8mb4`，时区配置正确（容器化部署依赖 `tzdata`，见 Docker 段）。
2. **Redis 6.x 及以上**。
3. **初始化数据库**：执行 `script/sql/ry_vue.sql`（建表 SQL 直接复用原 Java 项目，表结构一致）。
4. **改连接配置**：修改 `configs/application.yaml` 里的 `datasource`、`redis` 为本机配置。
5. **sa-token-go 依赖**：以本地检出路径 `replace`（见 `go.mod` 末尾）：

   ```
   github.com/sa-tokens/sa-token-go/core             => E:/WorkSpace/sa-token-go-main/core
   github.com/sa-tokens/sa-token-go/integrations/gin => E:/WorkSpace/sa-token-go-main/integrations/gin
   ...
   ```
   本机若可直连 goproxy，删掉该 `replace` 块并 `go get` 即可；否则需先在 `E:/WorkSpace/sa-token-go-main` 检出 sa-token-go
   源码。

完成上述步骤后执行依赖整理：

```bash
go mod tidy
go build ./...     # 全量编译，提交前必过
go vet ./...       # 静态检查，提交前必过
```

## 项目启动

### 单体启动

单体进程一次装配四个模块，适合本地开发与小规模部署：

```bash
go run ./cmd/standalone      # 监听 :8080
```

启动后接口直接以模块前缀暴露：

- 登录：`POST http://localhost:8080/auth/login`
- 图形验证码：`GET  http://localhost:8080/auth/code`
- 系统管理：`http://localhost:8080/system/...`（如 `/system/user`、`/system/role`）
- 对象存储 / 推送：`http://localhost:8080/resource/...`（推送端点 `/resource/message`）
- 监控：`http://localhost:8080/monitor/...`
- 活性探针：`GET http://localhost:8080/system/ping`（免鉴权，不碰 DB/Redis）

### 多模块启动

每个模块编译成独立 binary、独立进程。 **各模块统一监听 `:8080`**（为容器化对齐，靠网络命名空间隔离端口），本机同时起多个需临时改
`configs/<module>.yaml` 的 `addr`，否则抢端口：

```bash
go run ./cmd/modular/system    # :8080 → /system/*、/ping
go run ./cmd/modular/auth      # 改成 :8081 → /auth/*、/ping
go run ./cmd/modular/monitor    # 改成 :8082 → /monitor/*、/ping
go run ./cmd/modular/resource  # 改成 :8083 → /resource/*、/ping
```

modular 进程路由以 **空前缀**注册，探针统一为 `GET /ping`。生产环境由 nginx 按路径前缀分流并剥前缀（对齐 Java gateway
`StripPrefix=1`），前端始终打 `/auth`、`/system`、`/resource`、`/monitor` 前缀路径。

> **雪花 ID 注意**：各进程共用同一库，`snowflake.workerId` 必须互不相同，否则主键撞号。已分配：`auth=1`、`system=2`、
> `resource=3`、`standalone=0`。同模块起多副本时，副本须挂载各自的 yaml 覆盖 `workerId`。

## Docker 部署

> 构建上下文 **必须是仓库根目录**（需要 `go.mod` / `pkg` / `internal`），故 `-f` 指向 Dockerfile、`.` 作上下文。

### 单体部署

```bash
docker build -f cmd/standalone/Dockerfile -t ruoyi-go:standalone .
docker run -d -p 8080:8080 ruoyi-go:standalone
```

镜像内已打进 `configs`。改配置有两种方式：挂载覆盖整个目录 `-v ./configs:/app/configs:ro`，或只覆盖某个文件。
**环境变量改不了配置**——`pkg/config` 没开 `viper.AutomaticEnv`。

健康检查内置 `GET /system/ping`。

### 多模块部署

每个模块一个镜像，由 `MODULE` 构建参数选 `cmd/modular` 下的入口：

```bash
docker build -f cmd/modular/Dockerfile --build-arg MODULE=auth     -t ruoyi-go:auth .
docker build -f cmd/modular/Dockerfile --build-arg MODULE=system   -t ruoyi-go:system .
docker build -f cmd/modular/Dockerfile --build-arg MODULE=monitor  -t ruoyi-go:monitor .
docker build -f cmd/modular/Dockerfile --build-arg MODULE=resource -t ruoyi-go:resource .
```

四个容器各自监听 `:8080`，靠容器网络命名空间隔离，对外端口由 `docker run -p` 或上游 nginx upstream 区分。同一模块起多副本时，
`workerId` 必须互异——副本挂载各自的 yaml 覆盖（`-v ./configs/system-2.yaml:/app/configs/system.yaml:ro`）。

健康检查内置 `GET /ping`（四模块路由均空前缀注册，探针路径一致）。

镜像基础信息：

- 基础镜像 `alpine:3.20`，已装 `ca-certificates`（对象存储 / 阿里云短信 / 三方登录均 HTTPS 出站）与 `tzdata`（DSN 用了
  `loc=Local`，缺 tzdata 会按 UTC 写库，时间字段差 8 小时）。
- `CGO_ENABLED=0` 静态链接，`-trimpath -ldflags="-s -w"` 去绝对路径与符号表。
- 以非 root 用户 `ruoyi`（uid 10001）运行。

## 赞助作者

作者为兼职做开源，平时还需要工作，如果帮到了您可以请作者吃个盒饭。

<table>
  <tr>
    <td align="center"><img src="docs/donate/we_pay.jpg" width="240" alt="微信赞助码"/><br/>微信</td>
    <td align="center"><img src="docs/donate/ali_pay.jpg" width="240" alt="支付宝赞助码"/><br/>支付宝</td>
  </tr>
</table>

## Java 版本对比

| RuoYi (Java)                     | 本项目 (Go)                                     |
|----------------------------------|-------------------------------------------------|
| ruoyi-admin（聚合启动）          | `cmd/<module>/main.go`（每模块一进程）          |
| Controller                       | `internal/<module>/handler`                     |
| IXxxService / XxxServiceImpl     | `internal/<module>/service`                     |
| XxxMapper (MyBatis)              | `internal/<module>/repository`（GORM）          |
| domain / domain.bo / domain.vo   | `model`（entity / dto / vo）                    |
| ruoyi-common-*                   | `pkg/*`                                         |
| R&lt;T&gt; / TableDataInfo       | `pkg/response.R[T]` / `PageResult[T]`           |
| PageQuery                        | `pkg/repository.PageQuery`                      |
| sa-token（@SaCheckPermission）   | `pkg/middleware` 鉴权中间件 + sa-token-go       |
| @DataPermission（SpEL + 拦截器） | repository 层 GORM Scopes / Callback 手写       |
| @Log（AOP）                      | `pkg/oplog` 注解 + Recorder 回调                |
| hutool Tree                      | `pkg/tree`（扁平 JSON 契约自实现）              |
| ExcelUtil（easyexcel）           | `pkg/excel`（tag 驱动导出 / 导入）              |
| justauth（三方登录）             | `pkg/social`（自接 gitee/github/maxkey/topiam） |

迁移中两个最难处的处理方式：

1. **数据权限 `@DataPermission`**：Java 用 SpEL 注解 + MyBatis 拦截器动态拼 SQL 做部门 / 角色数据隔离。Go 无等价机制，改在
   repository 层用 GORM Scopes / Callback 手写数据权限过滤。
2. **sa-token 认证鉴权**：Java 用 sa-token 管登录态、`@SaCheckPermission` 校验权限。Go 用 sa-token-go（JWT + Redis
   存会话）复刻，权限校验改为 `pkg/middleware` 鉴权中间件查权限码。

## 目录结构

```
cmd/
  standalone/main.go          单体入口：auth+system+monitor+resource 同进程 :8080
  modular/
    auth/main.go              auth 进程入口
    system/main.go            system 进程入口
    monitor/main.go           monitor 进程入口
    resource/main.go          resource 进程入口
    Dockerfile                多模块通用镜像（--build-arg MODULE 选入口）
  standalone/Dockerfile       单体镜像
internal/                     按模块组织，严格三层
  <module>/
    router.go                 路由注册 RegisterRoutes(r, prefix)
    handler/                  HTTP 层：绑定参数、调 service、返回 response.R
    service/                  业务逻辑；system 的 service 导出供 auth 复用
    repository/               数据访问，只有这一层碰 GORM/DB
    model/                    entity(表) / dto(入参) / vo(出参)
pkg/                          可复用公共库
  response/config/database/redis/middleware/satoken/oplog/excel/tree/oss/push/...
configs/                      application.yaml(公共) + <module>.yaml(各进程端口/雪花ID)
script/sql/                   ry_vue.sql 建表 SQL（复用原项目表结构）
docs/CRUD-SPEC.md             增删改查落地模板与踩坑点（写 CRUD 前先读）
```

分层依赖单向：`handler → service → repository → model`，禁止反向依赖、禁止 handler 直连 repository。`pkg` 不能 import
`internal`（注解层落库等场景用 Recorder 回调让 internal 反向注册）。

## 开源协议

本项目基于 [MIT License](LICENSE) 开源，转载 / 二次开发请注明原作者与出处。

## 联系方式

- 邮箱：<401030526@qq.com>
- QQ：401030526


