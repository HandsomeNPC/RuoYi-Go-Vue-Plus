# pkg/ 外迁到 ruoyi-common-go（长周期任务）

> 本文是 **`pkg/` 拆分的进度表与规则书**。动 `pkg/` 下任何代码、或准备迁下一个包之前先读完本文。
>
> 这是一项跨多次会话的 11 步工作。 **每完成一步都要回来更新 §4 的状态表**，否则下次接手只能靠
> `ls ruoyi-common-go/` 猜进度，并重新推导一遍依赖图。
>
> 分层依赖、命名、雪花 ID、注释克制等 **全局约束见 `CLAUDE.md`**；CRUD 写法见 `docs/CRUD-SPEC.md`。
> 本文只讲外迁。

---

## 1. 为什么拆 + 目标形态

对照 Java 版的 `ruoyi-common`（24 个 maven 模块），把 `pkg/` 里通用的部分抽到独立仓库
`github.com/HandsomeNPC/ruoyi-common-go`，让 `redisx`/`gormx`/`excelx` 这类包能被其他 Go 项目直接复用； RuoYi 领域专属的部分下沉到
`internal/shared/`。

**终点**：开发者写新业务模块时按需引入 common 子包，然后 **只碰 `cmd/` 和 `internal/`**。

```
ruoyi-common-go/                      (单 module，不拆多 module)
├── core/                 errs+response 合并                  ✅
├── internal/cfgutil/     errMissing/errInvalid/msOrDefault   ← 第 5 步随 redisx 建
├── bcrypt/ excel/ regex/                                      ✅
├── ip/ jsonx/ snowflake/ strutil/ treeutil/ useragent/       ✅
├── captcha/              (当前只有 config.go，本体待迁)
├── i18n/                 引擎（词条留项目）—— 实现待重做，暂不迁
├── redisx/ cachex/ gormx/ encryptx/
├── mail/ sms/ social/ ossx/
└── web/ ratelimiter/ repeatsubmit/

RuoYi-Go-Vue-Plus/
├── cmd/
└── internal/
    ├── shared/           config constant enum i18n(词条) model satoken
    └── auth/ system/ monitor/ resource/      ← 开发者只碰这里
```

**`pkg/` 最终要整体消失**。Go 里 `pkg/` 的含义是"供外部 import"，拆完只剩 RuoYi 领域代码时这名字在说谎； 而且共享包若留在
`pkg/`，"新模块只写 `cmd/` + `internal/`" 就不成立。

> 完成信号可验证： **仓库里不再有 `pkg/` 目录**。比对着清单勾更可靠。

---

## 2. 核心规则：配置反转

> **进度更新**：原"唯一结构性阻塞点 `pkg/config`"已解决——`pkg/config` 整包删除，
> 19 个配置类型下沉到各 owning pkg（`pkg/redis`、`pkg/database`、`pkg/middleware` 等），
> `Init(cfg)` 收参不读全局；项目侧组装在 `internal/<module>/config`（每模块一份自包含 yaml）。
> **但下述四条规则仍适用于 common-go 外迁**：当前 owning pkg 仍保留 `Init(cfg)`+`Get()` 单例
> （rule 2 要求只留 `New(cfg)`），且 `internal/<module>/config` 是按模块拆的、非 `internal/shared/config`，
> 这些都要在外迁时再收敛。

原阻塞点回放（已解决，存档备查）：`pkg/config` 曾是中心枢纽，25 个包里 12 个直接 import 它， 用法是 `config.Get()`
读全局单例。任何包只要碰 `config`，就连带拖进 viper + 其余 16 个兄弟配置结构体 +
`i18n` + `snowflake`，让 `redis`/`database` 这类最该复用的包彻底不可复用。

解法已在 `ruoyi-common-go/captcha/config.go` 验证过。对比前后：

|                              | `pkg/config/captcha.go`（旧） | `ruoyi-common-go/captcha`（新） |
|------------------------------|-------------------------------|---------------------------------|
| 配置结构体归属               | 集中在 config 包              | **包自己持有**                  |
| viper                        | `setDefaults(v *viper.Viper)` | **无**                          |
| 全局单例                     | `config.Get().Captcha`        | **构造函数入参**                |
| 项目级约定（Redis key 前缀） | 从 `constant` 直接取          | **注入字段 + 零值兜底**         |

### 四条规则

1. **配置结构体归包自己**，保留 `mapstructure` tag 但 **不碰 viper**。
2. **只导出 `New(cfg) (T, error)`**，不要 `Init()` / `Get()` 全局单例。 （现有 12 个带 `func Init()` 的包都要按这个改：
   `captcha` `database` `encrypt` `mail` `oplog`
   `push` `ratelimiter` `redis` `repeatsubmit` `satoken` `sms` `social`）
3. **项目级约定改注入字段 + 零值兜底**。`pkg/constant/global_constants.go` 里的
   `CaptchaCodeKey`/`RepeatSubmitKey`/`RateLimitKey`/`SocialAuthCodeKey` 是 RuoYi 的 Redis 命名空间约定， **不该进
   common**。全部按 captcha 那样改成注入字段，零值时回退默认 —— `captcha`/`repeatsubmit`/
   `ratelimiter`/`social` 四个包对 `constant` 的依赖就一起消失了。
4. **组装层留项目**。`internal/shared/config` 负责把各 common 包的 `Config` 拼成项目的 yaml 结构，
   `Load()`/`Get()`/`validate()` 都在那。单例便利层（如果还想要）也放项目侧。

```go
// 库侧
package redisx
type Config struct {
    Host string `mapstructure:"host"`   // tag 留着，但不碰 viper
    ...
}
func New(cfg Config) (*redis.Client, error)   // 没有 Init()/Client()

// 项目侧 internal/shared/config
type Config struct {
    Redis   redisx.Config   `mapstructure:"redis"`
    Captcha captcha.Config  `mapstructure:"captcha"`
}
```

---

## 3. 依赖图快照

拆分前 `pkg/` 25 个包、约 1.96 万行（含测试）的 **生产依赖**（已排除 `_test.go`）。 这张图是推导拆分顺序的依据，重新跑一遍要扫
360 个 `.go` 文件，故存档在此。

```
零依赖:   errs  response  constant  i18n  bcrypt
          ↑
config ───┼── i18n                       ← viper，且是单一巨型 Config 结构体
   ↑
   ├── redis ── cache ── oss
   ├── database ── repository ── errs/response
   ├── middleware ── constant/errs/i18n/response
   ├── mail  sms  social  captcha  encrypt
   ├── ratelimiter  repeatsubmit ── middleware
   ├── satoken ── model(✗internal)  repository  enum
   ├── oplog ── middleware  satoken/loginhelper  enum
   └── push ── model(✗internal)  satoken/loginhelper
```

三个要点：

1. `config` 是枢纽（见 §2）。
2. **两处 `internal` 反向泄漏**，违反 `CLAUDE.md` 的 "pkg 不能 import internal"：
    - `pkg/model/login_user.go` → `internal/system/model/dto` 的 `RoleDTO`/`PostDTO`
    - `pkg/push/{manager,payload}.go` → 同上的 `PushPayloadDTO`
3. **好消息**：`middleware` 虽被 5 个包引用，但 `excel`/`ratelimiter`/`satoken` **只在测试里**用
   `middleware.Recover()`，生产代码无依赖。真实生产依赖只有 `oplog` 和 `repeatsubmit`。

业务代码对 `pkg/*` 的 import 次数（决定改动面大小）：

```
repository 52  errs 36  constant 35  response 29  database 23  loginhelper 22
redis 17  config 13  excel 10  cache 10  satoken 9  oplog 9  i18n 9  enum 9
repeatsubmit 8  social 5  ratelimiter 5  push 5  middleware 5  encrypt 5
captcha 5  sms 3  mail 3  oss 2  bcrypt 2  model 1
```

---

## 4. 进度表

状态：`✅ 已完成` / `🔜 下一步` / `⬜ 未开始`

| #  | 内容                                                        | 状态   | commit              | 关键约束                                                                                                                                                                                                             |
|----|-------------------------------------------------------------|--------|---------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 0  | `ip` `jsonx` `snowflake` `strutil` `treeutil` `useragent`   | ✅     | `1c4391b`…`1e93462` | 零依赖工具，原样搬                                                                                                                                                                                                   |
| 1  | **`core`**（`errs` + `response` 合并）                      | ✅     | `f77d604`           | `errs.New` → `core.NewError`；改写 55 文件、317 处调用                                                                                                                                                               |
| 2  | `bcrypt`、`regex`（从 `constant/regex.go` 抽通用正则）      | ✅     | 见本次提交          | 纯函数热身。包名用 `regex` 非 `regexx`——`regex` 不与任何包名冲突（stdlib 是 `regexp`），按命名规则不加 `x`。`constant/regex.go` 里 RuoYi 专属的（`RegexPermissionString`/`RegexDictionaryType`/`RegexStatus`）留项目 |
| 3  | `i18n` **引擎**                                             | ⏸️ 暂缓 |                     | 当前实现不够好，待重做后再迁。**不阻塞 excel/oplog**（只挡 config/enum/web，它们靠后）                                                                                                                               |
| 4  | `excel`                                                     | ✅     | 见本次提交          | 包名用 `excel` 非 `excelx`（不与 excelize 冲突）。生产依赖仅 `core`+gin+uuid+excelize。测试里的 `middleware.Recover()` 换成内联 `renderErrors`（库不能依赖主仓 middleware）                                          |
| 5  | `redisx` → `cachex`                                         | 🔜     |                     | 第一次完整实践配置反转；**此处建 `internal/cfgutil`**；缓存组名改注入                                                                                                                                                |
| 6  | `gormx`（`database` + `repository` 合并）                   | ⬜     |                     | 二者已有依赖关系。审计回调需注入 `UserIDFromContext func(ctx) (int64, bool)`。`PageResult[T]` 整组留这里（不拆去 `core`，否则和 `Page()`/`PageQuery` 分家）                                                          |
| 7  | `web`（`middleware` 改名）                                  | ⬜     |                     | 3482 行，体量最大但模式重复。保留 `XxxWithConfig(cfg)`、删掉读 `config.Get()` 的无参版                                                                                                                               |
| 8  | `encryptx`、`mail`、`sms`、`social`、`captcha` 本体、`ossx` | ⬜     |                     | `ossx` 只收 S3 包装；读 `sys_oss_config` 表选配置是业务，留项目                                                                                                                                                      |
| 9  | `ratelimiter`、`repeatsubmit`                               | ⬜     |                     | key 前缀注入；`repeatsubmit` 保留对 `web` 的依赖（要 `BodyBytes`）                                                                                                                                                   |
| 10 | 可选：`oplog`、`push` 通用部分                              | ⬜     |                     | `oplog` 需把操作者身份改注入 `IdentityResolver func(*gin.Context) Operator`、`BusinessType` 改 `int`；`push` 拆出通用 `wsmanager`（payload 用 `[]byte` 或泛型）                                                      |

### 留在项目侧的（不是缺陷，是正确结果）

拆完 `pkg/` 从 25 个包缩到 ~6 个， **不会归零** —— 这些就是 RuoYi 的领域知识：

| 包                        | 为什么留                                                                                                                                                |
|---------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `config`                  | 组装层：把各 common 包的 `Config` 拼成 yaml 结构，`Load`/`Get`/`validate` 都在这                                                                        |
| `constant`                | `cache_names.go`（`sys_config`/`sys_dict`…）、`system.go`（`SuperAdminUserID`/`AllPermission`）、`push.go`、`global_constants.go` 全是 RuoYi 表名和约定 |
| `enum`                    | `UserStatus`/`UserType`/`DeviceType`/`LoginType` 是 RuoYi 领域枚举                                                                                      |
| `i18n` 词条               | `user.not.exists` 等文案内容，引擎走 common                                                                                                             |
| `model`                   | `LoginUser` + 下沉来的 `RoleDTO`/`PostDTO`                                                                                                              |
| `satoken` + `loginhelper` | 会话模型绑死 `LoginUser`，且被业务引用 22 次，天然属 RuoYi 领域                                                                                         |

---

## 5. 两件要穿插做的事

### A. 清掉 `pkg` → `internal` 反向泄漏

违反 `CLAUDE.md` 约定，建议在第 2 步前后顺手做。把 `RoleDTO`/`PostDTO`（各 4-5 字段纯数据） 从 `internal/system/model/dto`
下沉到 `internal/shared/model`，方向就变成
`internal/system` → `internal/shared/model`，顺行。`PushPayloadDTO` 同理。

这是成本最低的修法 —— 另一条路是让 `push` 泛型化，收益不抵复杂度。

### B. `pkg/` 整体迁入 `internal/shared/`

涉及 112 个文件的 import 重写。 **不要单独做** —— 在每个包迁走时顺手把剩下的路径一起改， 反正那些文件当次就要动。

---

## 6. 模块策略：保持单 module

`ruoyi-common-go` 维持"一个 module + 多个子包"， **不要**学 Java 做一包一 module：

- Go 1.17+ 有 module graph pruning，只 import `strutil` 的消费者不会下载 aws-sdk/excelize。 单 module ≠ 强塞全部依赖。
- 多 module 的代价是实打实的：N 个 `go.mod` 要同步 bump、跨包改动要先 tag 再升版本、 本地开发满地 `replace`。一个人维护会被拖死。
- 真需要独立版本线的（如 `ossx` 想单独发布）再拆出去，是可逆的。

**命名**：仅在与第三方包名冲突时加 `x` 后缀（`redisx` 避 go-redis、`gormx` 避 gorm、`jsonx` 避
`encoding/json`），其余用裸名（`mail`/`sms`/`social`/`excel`/`captcha`）—— 这正是现有包名的规律。

### 本地开发：go.work 而非 replace

库未打 tag 发布，靠 `E:\WorkSpace\RuoYi-Plus\go.work` 串起两个 module（该文件 **不进版本库**， 父目录也不是 git 仓库）：

```
go 1.26.5

use (
	./RuoYi-Go-Vue-Plus
	./ruoyi-common-go
)

replace github.com/HandsomeNPC/ruoyi-common-go v0.0.0-00010101000000-000000000000 => ./ruoyi-common-go
```

两个坑，踩过了记在这：

1. **光有 `use` 不够**。主仓 `go.mod` 的 `require` 里留着 `go mod tidy` 写入的占位版本
   `v0.0.0-00010101000000-000000000000`，它仍会进模块图并触发联网解析（离线直接构建失败）。 所以还需要那条 `replace`。
2. **`replace` 必须带版本号**。写成无版本的通配形式会报
   `module ... is replaced at all versions in the go.work file` —— 同一模块不可既 `use` 又全版本 `replace`。

这样主仓 `go.mod` 里 **不留任何 `replace`**，别人 clone 下来走正常版本号解析。 将来库打了 tag，把 `require` 的占位版本换成真版本号、删掉
`go.work` 的 `replace` 即可。

---

## 7. 连带要改的约定

每步迁完都要检查这几处是否需要同步：

- **`CLAUDE.md` 规则 7**（当前写的是 "pkg 不能 import internal"）：`pkg/` 消失后要重写成
  "`internal/shared` 不能 import `internal/<module>`"。
- **`CLAUDE.md` 目录结构段** 和 **Java→Go 对应关系表**（`ruoyi-common-*` 那一行）。
- **`README.md`** 的目录树与对应关系表。
- **`docs/CRUD-SPEC.md`** 六文件模板里的所有 `pkg/` 路径。
- **`ruoyi-common-go/README.md`** 子包表格加新包（`bcrypt`/`regex`/`captcha` 均已补入）。
- 两个 `Dockerfile` 里提到 `pkg/config` 的注释（已随本次配置重构改为"各 owning pkg 的 Init (cfg)"，外迁时确认无残留）。
  `configs/application.yaml` 已删除，各模块自包含 yaml。
