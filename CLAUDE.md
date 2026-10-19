# CLAUDE.md

本文件指导 Claude Code 在本仓库中工作。这是 **RuoYi-Vue-Plus (Java/Spring Boot) 的 Go (Gin) 重写版**。

> **写增删改查代码前，先读 [`docs/CRUD-SPEC.md`](docs/CRUD-SPEC.md)。**
> 那里有六个文件的落地模板、注解/中间件对照表、以及踩过的坑（更新列取舍、
> 存在性判定不能用受影响行数、中间件顺序、LIKE 转义等）。照模板写，
> 不必再逐个翻 client/config 模块的源码。
>
> ⚠️ **但 CRUD-SPEC 的「导入路径」已过期**：它写的 `pkg/xxx`、
> `satoken.CheckPermission`、`oplog.Log`、`repository.PageQuery`、
> `encrypt.ApiEncrypt`、`middleware.Recover`、`escapeLike` 都不对。
> 真实路径见本文件的「公共库导入路径」一节。它的 **约定与踩坑部分仍然有效**，
> 只是符号名要按下表翻译。目录结构上，repository/service 也已改成
> 一层子包（`repository/notice/notice_repository.go`），不是扁平文件。

## 项目定位

- 参照 `E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus` 的功能，用 Go + Gin 重新实现。
- 架构： **多模块拆进程 + nginx 负载均衡**。每个业务模块编译成独立 binary、独立进程启动；另提供 `standalone`
  单体入口，四模块同进程，便于本地调试。
- 四个业务模块：`auth`（登录 / 验证码 / 三方绑定）、`system`（用户 / 角色 / 菜单 / 部门 / 字典 / 参数 / 公告 / 客户端 /
  社交账号）、`monitor`（在线用户 / 登录日志 / 操作日志 / 缓存监控）、`resource`（OSS 对象存储 / 消息推送 / 短信 / 邮箱验证码）。
- 数据库： **MySQL + GORM**，所有进程共用同一个库（`ry-cloud`）。Redis 做会话 / 缓存 / 分布式锁 / 验证码 / 推送分发。

## 目录结构与分层

标准 Go 布局，`internal` 按模块组织，严格三层：

```
cmd/
  standalone/main.go        单体入口：auth+system+monitor+resource 同进程 :8080
  modular/
    <module>/main.go        多模块入口，每模块一进程（auth/system/monitor/resource）
    Dockerfile              多模块通用镜像（--build-arg MODULE 选入口）
  standalone/Dockerfile     单体镜像
internal/<module>/
  router.go                 路由注册 RegisterRoutes(r *gin.Engine, prefix string)
                            + InitRouter() *gin.Engine（给 modular 进程用）
  server.go                 InitServer(r)：读本模块 server 配置并 r.Run(addr)
  config/                   本模块的 viper 配置包：Load(path) / Get()
  handler/                  HTTP 层：绑定参数、调 service、返回 response.R
  service/<xxx>/            业务逻辑，一个业务一个子包；system 的 service 导出供 auth 复用
  repository/<xxx>/         数据访问，只有这一层碰 GORM/DB
  model/                    entity(表) / bo(入参) / vo(出参)
configs/                    各模块自包含 <module>.yaml（server/snowflake/datasource/redis
                            + 本模块用到的子配置）；无公共 application.yaml
script/sql/                 ry_vue.sql 建表 SQL（复用原项目表结构）
                            postgre/postgres_ry_vue.sql 为 PG 版
docs/                       CRUD-SPEC.md（写 CRUD 前先读）
```

> **本仓没有 `pkg/` 目录。** 公共库已外迁到独立模块
> `github.com/HandsomeNPC/ruoyi-common-go`。`docs/CRUD-SPEC.md` 里所有 `pkg/xxx`
> 的写法都是历史残留，真实导入路径见下面的「公共库导入路径」一节。

> nginx 负载均衡 / 前缀剥离的配置不在仓库内（各 modular 进程路由以空前缀注册，由上游网关 StripPrefix=1 剥前缀），需要时自行补
> `deploy/` 目录。

**分层依赖是单向的**：`handler → service → repository → model`。禁止反向依赖，禁止 handler 直连 repository。公共库 （
`ruoyi-common-go`）不能依赖本仓 `internal`（操作日志等落库场景用 Recorder 回调让 internal 反向注册， 见
`operlog.Init(operlogservice.RecordOper)`）。

## 公共库导入路径（写代码前先对照，别照 CRUD-SPEC 的旧写法）

全部在 `github.com/HandsomeNPC/ruoyi-common-go/` 之下。常用别名按本仓现有代码的习惯：

| 用途                                             | 真实路径                                                            | 本仓习惯别名  |
|--------------------------------------------------|---------------------------------------------------------------------|---------------|
| 统一响应 `R[T]` / `Ok` / `Fail`                  | `ruoyi-core/model/response`                                         | `response`    |
| 业务错误 `errs.New(code,msg,detail)`             | `ruoyi-core/errs`                                                   | `errs`        |
| 分页 `PageQuery` / `PageResult[T]` / `Page[T]()` | `ruoyi-core/model/page`                                             | `pkgpage`     |
| `SelectPage` / `EscapeLike` / `LogicDelete`      | `ruoyi-core/repository`                                             | `pkgrepo`     |
| `BaseEntity` / `LoginUser`                       | `ruoyi-core/model`                                                  | `coremodel`   |
| 登录态 helper `GetLoginUser` / `GetUserID`       | `ruoyi-core/utils/loginhelper`                                      | `lh`          |
| 登录校验中间件 `CheckLogin()`                    | `ruoyi-core/middleware/checklogin`                                  | `checklogin`  |
| 权限校验 `CheckPermission` / `CheckRole`         | `ruoyi-core/middleware/permission`                                  | `permission`  |
| 操作日志 `Log(title, BusinessTypeX)`             | `ruoyi-core/middleware/operlog`                                     | `operlog`     |
| 审计 ctx 注入 `AuditContext()`                   | `ruoyi-core/middleware/audit`                                       | `audit`       |
| 全局异常兜底 `Recover()`                         | `ruoyi-core/middleware/recover`                                     | `recover`     |
| 404/405 兜底 `Register(r)`                       | `ruoyi-core/middleware/notfound`                                    | `notfound`    |
| 防重提交 / 限流 / 加密                           | `middleware/{repeatsubmit,ratelimiter,apiencrypt}`                  | 同名          |
| CORS / XSS / TraceID / 访问日志 / 可重读 body    | `middleware/{cors,xss,trace,accesslog,repeatablebody}`              | 同名          |
| DB / Redis / 雪花 / JSON codec                   | `database` / `redis` / `snowflake` / `jsonx`                        | 同名          |
| Excel 导出                                       | `excelhelper`                                                       | `excelhelper` |
| 其它                                             | `cache` / `strutil` / `treeutil` / `bcrypt` / `oss` / `satoken` ... | 同名          |

鉴权插件来自另一个模块：`github.com/HandsomeNPC/sa-token-go/integrations/gin`（别名 `sagin`）。

⚠️ **绝不要直接挂 `sagin.CheckPermission()` / `sagin.CheckLogin()`**，要用
`ruoyi-core/middleware/permission` 和 `checklogin` 的包装版。裸的 sa-token 版本自行
`c.JSON` 并带 **真实 HTTP 401**，而前端把"跳登录页"写在 axios 成功分支里，真 401 落到 error 分支 → 只弹 toast、不跳转。本项目契约是
**HTTP 状态恒 200 + body `code`**。

## 关键约定（务必遵守）

1. **统一响应**：handler 一律返回 `response.R[T]` / `page.PageResult[T]`，用 `response.Ok/OkVoid/Fail/FailCode` 构造。 不要手写
   `gin.H` 拼响应。 **HTTP 状态恒 200**，业务码只在 body 的 `code` 里 （
   `CodeSuccess=200 / BadRequest=400 / Unauthorized=401 / Forbidden=403 / NotFound=404 / Fail=500 / Warn=601`）。
2. **auth 复用 system 走 in-process**：`internal/auth/service` 直接 `import internal/system/service`，同进程函数调用， **不走
   HTTP**。auth 进程也连同一数据库。
3. **只有 repository 层接触 GORM**。service 拿到的是 model，不感知 SQL。
4. **模块隔离**：`internal/system` 与 `internal/auth` 之间只能通过 service 层导出的接口交互，不要跨模块 import
   handler/repository/model 内部。
5. **命名**：包名小写无下划线；文件名 `xxx_handler.go`/`xxx_service.go`/`xxx_repository.go`；实体对应表名（`SysUser` → 表
   `sys_user`）。
6. **错误处理**：service 返回 `error`，由 handler `_ = c.Error(...)` 上交， **绝不自己 `c.JSON` 错误响应**； 全局兜底是
   `ruoyi-core/middleware/recover` 的 `Recover()`，它只在 `!Written()` 时渲染——抢先写字节会让后续错误被静默吞掉。 构造错误用
   `errs.New(code, msg, detail)`，第三参 `detail` **只进日志不回前端**。
7. **注释克制**：只写代码读不出来的“为什么”（非显然的取舍、踩坑点、与惯常做法的有意差异），不写代码已自 解释的“是什么”。
   **禁止**堆砌“对照 Java XxxAspect.doBefore / 对照 Xxx.yyy”这类方法级映射说明，也不在函数 上方写逐行重复实现步骤的流水账。包/函数
   doc 一句话点明职责即可，复杂逻辑用一两句说清动机。
8. **雪花 ID 的 JSON 形态由全局 codec 兜住**：VO/BO/entity 的 ID 字段一律用 **裸 `int64`**， **不要**加 `,string`
   tag，也不要为它套自定义类型。`github.com/HandsomeNPC/ruoyi-common-go/jsonx` 已接管 gin 的 JSON codec， 按值判断——超出 JS
   安全整数范围（±9007199254740991）自动转字符串出参、且入参同时认字符串与数字， 与 Java `BigNumberSerializer` 等价。加
   `,string` 反而会 **拒收数字**形态。 新增进程入口须在 `main()` 开头调 `jsonx.OverrideGinCodec()`；自定义 `MarshalJSON`
   内部用
   `jsonx.Marshal`
   而非 `encoding/json`，否则会绕过该编码器。
9. **审计字段自动填充，不要手填**。实体在 **末尾**嵌 `coremodel.BaseEntity`
   （`CreateDept/CreateBy/CreateTime/UpdateBy/UpdateTime`， **`CreateBy` 是 `int64` 用户 ID 不是用户名**）。 填充由
   `ruoyi-core/repository` 的 GORM Callback 完成， **按列名识别**（无这些列的表天然不受影响）。 回调在该包的
   `audit_register.go` 里通过 `database.RegisterOpenHook("ruoyi:audit", ...)` 自注册， repository 层只要 import 了这个包（用
   `SelectPage` 就会 import），`database.Init` 时回调自动挂上， **业务侧无需任何注册动作**。两个强约束：
    - 路由组必须挂 `audit.AuditContext()`，且 **排在 `plugin.TokenInterceptor()` 之后**（它取的登录态依赖后者解析出的
      token）；
    - **repository 每次查询都要 `.WithContext(ctx)`**，否则回调拿不到登录态，`create_by` 回落 `-1`。

   插入时也不要手填 `create_*`，更新列里不要带 `update_*`。
10. **软删除挂在字段类型上，不是 `gorm.DeletedAt`**。需要时在实体末尾嵌 `pkgrepo.LogicDelete`
    （`LogicDelete` 在前、`BaseEntity` 在后）。`DelFlag` 自己实现了 Query/Update/Delete 三组 Clauses， 查询与更新自动追加
    `del_flag = '0'`，`Delete` 改写为 `UPDATE ... SET del_flag = '1'`，`Unscoped()` 绕过。 值是 char (1) 的 `'0'`/`'1'`
    ，不是时间戳。 **不是每张表都有**——先查 `script/sql/ry_vue.sql` 的建表语句确认。 ⚠️ 配套约束：`Model()` 不能省（del_flag
    过滤要先解析实体 schema 才生效），`Count`/`SelectList`/`SelectPage` 全都要带。
11. **各进程 `snowflake.workerId` 必须唯一**。现有分配见 `configs/system.yaml` 顶部注释：
    `standalone=0 / auth=1 / system=2 / monitor=3 / resource=4`。所有进程共用一个库，撞号会主键冲突。 新增模块取未用值。

## 新增一个业务模块要改什么

新建：

```
internal/<m>/config/config.go     仿 internal/monitor/config/config.go（最精简版本）
internal/<m>/router.go            RegisterRoutes(r, prefix) + InitRouter()
internal/<m>/server.go            照抄 internal/system/server.go
internal/<m>/{model,repository,service,handler}/...
cmd/modular/<m>/main.go           照抄 cmd/modular/system/main.go（全文 25 行）
configs/<m>.yaml                  自包含；snowflake.workerId 取未用值
script/sql/                       建表 + sys_menu/按钮插入 + sys_role_menu 授权
```

修改：`cmd/standalone/main.go` 两处（追加 `<m>config.Load(path)`；追加
`<m>.RegisterRoutes(r, "/<m>")`， **必须在 `notfound.Register(r)` 之前**）、
`configs/standalone.yaml`（如有模块专属子配置）、本文件的模块清单与 workerId 表。

**`main()` 的初始化顺序是硬依赖**：
`config.Load` → `jsonx.OverrideGinCodec`（须在首个 `c.JSON`/参数绑定之前） → `database.Init` → `redis.Init` →
`satoken.Init(cfg, redis.Client())`
→ `snowflake.Init` → `repeatsubmit.Init(redis.Client(), satoken.TokenName())`
→ `operlog.Init(recorder)`（要写库，依赖 database + snowflake）→ `InitRouter` → `InitServer`。

**全局中间件顺序**（standalone 与各 `InitRouter` 一致）：
`Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS`，最后 `notfound.Register`。 **没有 I18n 中间件**（CRUD-SPEC
提到的是旧设计）。

## 常用命令

```bash
go build ./...              # 全量编译（提交前必过）
go vet ./...                # 静态检查（提交前必过）
go run ./cmd/standalone      # 单体进程 auth+system+monitor+resource :8080
go mod tidy                 # 引入新依赖后整理

# modular 各模块统一监听 :8080（为容器化对齐），故本机同时起多个需临时改
# configs/<module>.yaml 的 addr，否则抢端口。
go run ./cmd/modular/system  # 启动 system 进程 :8080

# 容器构建：上下文必须是仓库根目录
docker build -f cmd/standalone/Dockerfile -t ruoyi-go:standalone .
docker build -f cmd/modular/Dockerfile --build-arg MODULE=system -t ruoyi-go:system .
```

新增业务代码引入 GORM/redis/viper 等依赖后，务必 `go mod tidy` 并确认 `go build ./...` 通过。

## 从 Java 原项目迁移的对应关系

| RuoYi (Java)                   | 本项目 (Go)                                         |
|--------------------------------|-----------------------------------------------------|
| ruoyi-admin(聚合启动)          | cmd/modular/<module>/main.go(每模块一进程)          |
| Controller                     | internal/<module>/handler                           |
| IXxxService / XxxServiceImpl   | internal/<module>/service/<xxx>(包级函数，非结构体) |
| XxxMapper (MyBatis)            | internal/<module>/repository/<xxx> (GORM)           |
| domain / domain.bo / domain.vo | model(entity / bo / vo)                             |
| ruoyi-common-*                 | 外部模块 github.com/HandsomeNPC/ruoyi-common-go/*   |
| R<T> / TableDataInfo           | response.R[T] / page.PageResult[T]                  |
| PageQuery                      | page.PageQuery                                      |
| BaseEntity                     | coremodel.BaseEntity（嵌在实体末尾）                |
| @SaCheckPermission             | permission.CheckPermission(...)                     |
| @Log(title, businessType)      | operlog.Log(title, operlog.BusinessTypeXxx)         |
| @RepeatSubmit                  | repeatsubmit.RepeatSubmit(interval, msg)            |
| @ApiEncrypt                    | apiencrypt.APIEncryptWithConfig(cfg)                |
| @ExcelProperty                 | VO 字段的 `excel` tag（值逐字抄 Java）              |
| BigNumberSerializer            | jsonx.OverrideGinCodec()                            |

## 迁移中最需要注意的两个难点

1. **数据权限 `@DataPermission`**：Java 用 SpEL 注解 + MyBatis 拦截器动态拼 SQL 做部门/角色数据隔离。 Go 无等价机制，需在
   repository 层用 **GORM Scopes / Callback** 手写数据权限过滤。这是移植工作量最大处。
2. **sa-token 认证鉴权**：Java 用 sa-token 管登录态、`@SaCheckPermission` 校验权限。 Go 用
   `github.com/HandsomeNPC/sa-token-go`（JWT + Redis 存会话）复刻，权限校验走
   `ruoyi-core/middleware/permission` 的 `CheckPermission` / `CheckRole`。 注意 **单个 `CheckPermission(a, b)` 内多值是
   OR**（任一命中放行），而 **串联两个中间件是 AND**（如菜单接口的 `CheckRole(超管) + CheckPermission(...)`）。 取当前登录用户用
   `loginhelper.GetLoginUser(c)` / `GetUserID(c)` 等， **只能在 handler 层调**
   （它们要 `*gin.Context`）；service 层只有 `context.Context`，操作者靠审计回调自动填， 确需知道操作者时由 handler 显式传参。

## 参考

- 原项目：`E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus`（Java 源码，迁移时对照它的业务逻辑）
- 建表 SQL：本仓库 `script/sql/ry_vue.sql`（MySQL，表结构直接复用原项目）； PG 版在 `script/sql/postgre/postgres_ry_vue.sql`
  。无 migration 框架、无按模块拆分的增量脚本。
- 公共库源码：`github.com/HandsomeNPC/ruoyi-common-go`（本机模块缓存在 `$GOPATH/pkg/mod` 下，
  审计回调、软删除、分页等实现都在那里，不确定行为时直接读源码而非猜）
