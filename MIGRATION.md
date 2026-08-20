# 迁移计划：RuoYi-Vue-Plus (Java) → Go (Gin)

将 `E:\WorkSpace\RuoYi-Plus\RuoYi-Vue-Plus` 逐步重写为 Go 实现。本计划按 **依赖顺序**推进：
先地基（公共库），再核心闭环（登录），再业务模块，最后横切难点。

> 现状：骨架已就绪（`go build ./...` 通过），仅 `pkg/response` 有实现，其余为占位。

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
| 配置加载     | `pkg/config`     | viper 读 application.yaml + <module>.yaml，结构体绑定    |
| DB 初始化    | `pkg/database`   | GORM + MySQL 驱动，连接池，全局 *gorm.DB                 |
| Redis 初始化 | `pkg/redis`      | go-redis 客户端                                          |
| 统一响应     | `pkg/response`   | ✅ 已完成（R / PageResult）                              |
| 全局中间件   | `pkg/middleware` | Recover(全局异常→response.Fail)、CORS、请求日志、TraceID |
| 常量         | `pkg/constant`   | 从原 common-core/constant 移植需要的常量                 |

**依赖引入**：
`go get gorm.io/gorm gorm.io/driver/mysql github.com/redis/go-redis/v9 github.com/spf13/viper github.com/golang-jwt/jwt/v5`
，然后 `go mod tidy`。

**验收**：`cmd/system` 能加载配置、连上 DB/Redis、挂上中间件启动。

---

## 阶段 1：认证闭环（登录能跑通）— 最高优先级

打通 auth ←in-process→ system 的完整链路，验证架构成立。预计 3-4 天。

| 顺序 | 任务                                               | 位置                                             |
|------|----------------------------------------------------|--------------------------------------------------|
| 1    | `SysUser` 实体 + 表映射                            | `internal/system/model`                          |
| 2    | UserRepository：按用户名查用户                     | `internal/system/repository`                     |
| 3    | UserService：`GetByUsername`                       | `internal/system/service`（导出给 auth）         |
| 4    | JWT 签发/校验 + Redis 会话                         | `pkg/auth`                                       |
| 5    | AuthService：`Login`（校验密码→签发 token→存会话） | `internal/auth/service`（import system service） |
| 6    | AuthHandler：`POST /auth/login` `/logout`          | `internal/auth/handler`                          |
| 7    | 鉴权中间件：解析 token→校验会话                    | `pkg/middleware`                                 |

**验收**：`POST /auth/login` 返回 token，带 token 访问受保护接口通过。 **架构验证点**——此阶段确认「auth 进程内嵌 system
service、无网络调用」成立。

> 先只做 **密码登录（PasswordAuthStrategy）**。短信/邮箱/社交/小程序登录留到阶段 4。

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

- **M1**（阶段 0-1）：登录闭环跑通，架构验证 ✅ 关键节点
- **M2**（阶段 2）：RBAC 完整，可管理用户/角色/菜单
- **M3**（阶段 3）：system 模块功能对齐
- **M4**（阶段 4）：数据权限 + 多登录方式，安全能力对齐
- **M5**（阶段 5）：其余模块按需补齐
