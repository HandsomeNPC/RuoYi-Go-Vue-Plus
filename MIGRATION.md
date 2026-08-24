# 迁移计划：RuoYi-Vue-Plus (Java) → Go (Gin)

将 `E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus` 逐步重写为 Go 实现。本计划按 **依赖顺序**推进：
先地基（公共库），再核心闭环（登录），再业务模块，最后横切难点。

> 现状：阶段 0（pkg 公共层）与阶段 1（认证闭环）已完成，M1 达成 —— 登录/登出跑通、鉴权中间件生效、
> 「auth 进程内嵌 system service 无网络调用」已验证。下一步是阶段 2 的 RBAC。

## 原项目规模（system 模块）

- **15** 个 Controller（User/Role/Menu/Dept/Post/Dict/Config/Notice/Oss/Client/Social/Message/Profile...）
- **18** 个 Service 接口
- **22** 个数据库实体（sys_user / sys_role / sys_menu / sys_dept / sys_user_role ...）
- 登录入口在 ruoyi-admin：`AuthController` + 多种策略（Password/Sms/Email/Social/Xcx）

---

## 阶段 0：地基（pkg 公共层）— 必须最先做

无地基无法写任何业务。预计 2-3 天。

| 任务         | 落地位置         | 说明                                                     |
|--------------|------------------|----------------------------------------------------------|
| 配置加载     | `pkg/config`     | ✅ viper 读 application.yaml + <module>.yaml，结构体绑定 |
| DB 初始化    | `pkg/database`   | ✅ GORM + MySQL 驱动，连接池，全局 *gorm.DB              |
| Redis 初始化 | `pkg/redis`      | ✅ go-redis 客户端                                       |
| 统一响应     | `pkg/response`   | ✅ 已完成（R / PageResult）                              |
| 全局中间件   | `pkg/middleware` | ✅ 9 个全部落地，见下                                    |
| 加解密原语   | `pkg/encrypt`    | ✅ AES-ECB / RSA / base64，对应 EncryptUtils             |
| 登录态原语   | `pkg/auth`       | ✅ JWT 签发/校验 + Redis 会话 + BCrypt，对应 sa-token    |
| 常量         | `pkg/constant`   | ✅ 从原 common-core/constant 移植需要的常量              |
| 国际化       | `pkg/i18n`       | ✅ 词条表 + Msg(ctx,...)，对应 MessageUtils              |

全局中间件进度（详见 `pkg/middleware/README.md`）：`Recover` / `CORS` / `TraceID` / `ApiEncrypt` /
`RepeatableBody` / `AccessLog` / `XSS` / `I18n` / `Auth` **已全部落地**，由 `middleware.Register(r)` 统一按序注册
（两个入口各一行调用）。 各中间件的配置在
`configs/application.yaml` 的 `middleware` 段，结构体定义在 `pkg/config/middleware.go`， 运行期从 `config.Get()` 读 —— 因此
**`config.Load` 必须早于 `middleware.Register`**（`Auth` 还要求先 `redis.Init`）。

> `ApiEncrypt`（接口加解密，对应 Java 的 `CryptoFilter`） **必须排在 `RepeatableBody` 之前**，
> 否则日志只能看到密文、脱敏失效、handler 还绑不到参数。它是唯一带 `enabled` 开关的中间件。
> 请求解密已按原项目协议对齐并有 FIPS-197 向量交叉验证； **PKCS#7 填充与 base64 那两层仍未跨语言核实**，
> 首次前后端联调时应重点验。响应加密照 `EncryptResponseBodyWrapper` 复刻但默认关闭 ——
> 原项目 4 处 `@ApiEncrypt` 全是 `response=false`，那条链路从未被启用过。

**依赖引入**：
`go get gorm.io/gorm gorm.io/driver/mysql github.com/redis/go-redis/v9 github.com/spf13/viper github.com/golang-jwt/jwt/v5 golang.org/x/crypto/bcrypt`
，然后 `go mod tidy`。测试用 `go get -t github.com/alicebob/miniredis/v2 github.com/glebarez/sqlite`
（内存 Redis 与纯 Go 的 SQLite，让会话 TTL 与登录链路能脱离外部环境断言）。

**验收**：`cmd/system` 能加载配置、连上 DB/Redis、挂上中间件启动。✅

---

## 阶段 1：认证闭环（登录能跑通）— 最高优先级 ✅ 已完成

打通 auth ←in-process→ system 的完整链路，验证架构成立。

| 顺序 | 任务                                                  | 位置                                             |
|------|-------------------------------------------------------|--------------------------------------------------|
| 1    | ✅ `SysUser` 实体 + 表映射                            | `internal/system/model/user.go`                  |
| 2    | ✅ UserRepository：按用户名查用户                     | `internal/system/repository/user_repository.go`  |
| 3    | ✅ UserService：`GetByUsername`                       | `internal/system/service`（导出给 auth）         |
| 4    | ✅ JWT 签发/校验 + Redis 会话                         | `pkg/auth`（token/session/claims/password）      |
| 5    | ✅ AuthService：`Login`（校验密码→签发 token→存会话） | `internal/auth/service`（import system service） |
| 6    | ✅ AuthHandler：`POST /auth/login` `/logout`          | `internal/auth/handler` + `router.go`            |
| 7    | ✅ 鉴权中间件：解析 token→校验会话                    | `pkg/middleware/auth.go` + `ip.go`               |

顺带做了计划外但必需的几项：`SysClient` 实体与 repository/service（登录要按 `clientId` 查授权类型与超时配置）、
`repository/scope.go` 的 `NotDeleted()`（Java 的 `@TableLogic` 在 GORM 没有等价物，漏一处就是一条能查出已删数据的路径）、
`pkg/config` 的 `middleware.auth.*` 与 `user.password.*` 两段配置。

**验收结果**（真实 MySQL + Redis，种子数据取原项目 `script/sql/ry_vue.sql`）：

- `POST /auth/login`（按 `apiEncrypt` 协议加密）返回 token、`expire_in=604800`、`client_id` ✅
- 带 token + 匹配的 `clientid` 访问 **system 进程**的受保护接口 → 200 ✅ （ **跨进程验证**：auth 签的 token 在 system
  那边通过，靠的是同一个 `jwt.secret` 与同一个 Redis 会话）
- 不带 token / 不带 clientid / clientid 不匹配 → `{"code":401,...}`，HTTP 状态码恒 **200** ✅
- 未注册的乱路径 → **404 而非 401**（对齐 Java 的 `AllUrlHandler`）✅
- 连错 5 次密码 → 「密码输入错误5次，账户锁定10分钟」，此后即使密码正确也拒绝；计数键 TTL 每次失败重置为 10 分钟 ✅
- 登出后原 token 再访问 → 401（Redis 会话已删）✅

> **架构验证点（M1）已确认成立**：`internal/auth/service` 直接 `import internal/system/service`，
> 同进程函数调用、 **无任何 HTTP 客户端**。auth 进程因此也连同一个数据库。

联调工具在 `tools/e2elogin` 与 `tools/e2elockout`（ **临时联调用，非产品代码**）——
`configs/application.yaml` 里 `apiEncrypt.enabled=true` 且 `/auth/login` 在强制加密清单里， 用 curl 发明文会被 403
拒掉，故需要按协议加密的客户端。这也顺带核实了 README 里「PKCS#7 填充与 base64 那两层仍未跨语言核实」中 Go
侧自洽的那一半（前后端联调时仍需与真实前端对齐）。

> 先只做 **密码登录（PasswordAuthStrategy）**。短信/邮箱/社交/小程序登录留到阶段 4。
> 本阶段有意未做、已在代码里留 TODO 指向对应阶段的：验证码校验（阶段 3，需先有生成接口，
> 原项目 `captcha.enable` 默认 false）、登录日志落库（`sys_login_info`，阶段 3，现只打日志）、
> `menuPermission`/`rolePermission`/`roles`/`posts`/`deptName`（阶段 2，依赖 `sys_menu`/`sys_role`/`sys_post`/`sys_dept`）。

---

## 阶段 2：system 权限核心（RBAC）

用户/角色/菜单/部门是一切权限的基础。预计 1-1.5 周。

| 任务     | 实体                                   | 接口要点                              |
|----------|----------------------------------------|---------------------------------------|
| 用户管理 | sys_user, sys_user_role, sys_user_post | CRUD、分页、重置密码、分配角色        |
| 角色管理 | sys_role, sys_role_menu, sys_role_dept | CRUD、分配菜单权限、数据权限范围      |
| 菜单管理 | sys_menu                               | 树形结构、路由构建、按角色查菜单      |
| 部门管理 | sys_dept                               | 树形结构、级联                        |
| 岗位管理 | sys_post                               | CRUD                                  |
| 权限校验 | —                                      | 权限码中间件、`getRouters`、`getInfo` |

**验收**：登录后能拿到用户菜单树、权限码；用户/角色/菜单 CRUD 完整。

---

## 阶段 3：system 其余业务

| 任务          | 实体                         |
|---------------|------------------------------|
| 字典管理      | sys_dict_type, sys_dict_data |
| 参数配置      | sys_config                   |
| 通知公告      | sys_notice                   |
| 操作/登录日志 | sys_oper_log, sys_login_info |
| 客户端管理    | sys_client                   |
| OSS 文件      | sys_oss, sys_oss_config      |
| 个人中心      | (基于 sys_user)              |

预计 1-1.5 周。日志类可先做写入，查询后补。

---

## 阶段 4：横切难点（工作量与风险最高）

这两块没有现成对应物，是整个迁移的 **关键风险**，建议独立排期、充分测试。

### 4.1 数据权限（对应 @DataPermission）

- Java：SpEL 注解 + MyBatis 拦截器按部门/角色动态拼 SQL。
- Go 方案：在 repository 层用 **GORM Scopes** 注入数据权限条件；从上下文取当前用户的数据范围（全部/本部门/本部门及以下/仅本人/自定义）。
- 建议放 `pkg/middleware` 存用户数据范围到 context，`repository` 层统一 Scope 应用。
- **风险**：涉及所有带数据权限的查询，需逐接口验证隔离正确。

### 4.2 多种登录策略

- 短信、邮箱、社交（OAuth）、小程序登录。
- 抽象成 `AuthStrategy` 接口，各策略独立实现，对齐 Java 的策略模式。

---

## 阶段 5：其他模块（按需）

原项目还有 gen (代码生成)、job (定时任务)、workflow (工作流)、ai 等模块。按业务优先级决定是否迁移，每个都新建
`cmd/<module>` + `internal/<module>`。

- **job**：定时任务务必独立进程单实例，避免多副本重复执行。

---

## 通用迁移规范（每个接口都遵守）

1. 对照 Java 源码理解业务， **不照搬代码结构**，用 Go 惯用法重写。
2. 顺序：`model(实体) → repository → service → handler → router 注册`。
3. 每完成一个 Service，跑 `go build ./... && go vet ./...`。
4. 表结构直接复用原项目 `script/sql/`，实体字段与表列对齐。
5. handler 只做参数绑定 + 调用 service + 返回 `response.R`，不写业务逻辑。
6. 引入新依赖后 `go mod tidy`。

## 里程碑

- **M1**（阶段 0-1）：登录闭环跑通，架构验证 ✅ **已达成**
- **M2**（阶段 2）：RBAC 完整，可管理用户/角色/菜单
- **M3**（阶段 3）：system 模块功能对齐
- **M4**（阶段 4）：数据权限 + 多登录方式，安全能力对齐
- **M5**（阶段 5）：其余模块按需补齐
