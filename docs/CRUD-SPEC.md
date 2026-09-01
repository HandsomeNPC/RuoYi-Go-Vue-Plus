# CRUD 实现规范（AI 必读）

> 本文是 **增删改查模块的落地模板**。写任何 `sys_xxx` 的 CRUD 之前先读完本文，
> 照抄模板 + 查对照表即可， **不需要再去翻 client/config 模块的源码**。
>
> 分层依赖、命名、雪花 ID、注释克制等 **全局约束见 `CLAUDE.md`**，本文不重复，只讲 CRUD 的具体写法。
>
> 参考实现（本文所有片段都取自它们，需要看完整上下文时再读）：
> `sys_config`（无逻辑删除 + 缓存 + 内置保护）、`sys_client`（有逻辑删除 + 状态开关）。

---

## 0. 动手前先确认三件事

| 要确认的            | 怎么查                                                                                            | 影响什么                                                               |
|---------------------|---------------------------------------------------------------------------------------------------|------------------------------------------------------------------------|
| 表有没有 `del_flag` | 原项目 `script/sql/ry_vue.sql` 的 `create table`                                                  | 决定 entity 是否嵌 `repository.LogicDelete`，删除是物理还是逻辑        |
| Java 侧的过滤语义   | 原项目 `XxxServiceImpl.buildQueryWrapper`                                                         | `likeIfText` → LIKE，`eqIfText` → `=`，`betweenParams` → 闭区间        |
| 前端实际调用形态    | `ruoyi-plus-vben5-main/apps/web-antd/src/api/system/<模块>/index.ts` 与同级 `views/.../index.vue` | 导出是 POST form、日期区间摊平成 `params[beginTime]`、哪些字段真会回传 |

**别跳过第三条。** Java Controller 的方法签名不等于前端的调用形态，
`updateByKey` 就是典型：Java 复用 `SysConfigBo`，但前端只回传 2 个字段。

---

## 1. 六个文件，固定顺序

按 `model → repository → service → handler → router → test` 的顺序写，每步 `go build ./...` 一次。

```
internal/system/model/entity 或 model/sys_xxx.go   实体（多数已存在，先 ls 确认）
internal/system/model/bo/sys_xxx_query_bo.go       查询条件（新建，勿复用写入 BO）
internal/system/model/vo/sys_xxx_vo.go             出参 + excel tag（多数已存在，补 tag）
internal/system/repository/xxx_repository.go       数据访问
internal/system/service/xxx_service.go             业务逻辑
internal/system/handler/xxx_handler.go             HTTP 层
internal/system/router.go                          注册路由（改现有文件）
```

`model/bo/conv.go`、`model/vo/conv.go` 里的 goverter 接口 **多数已声明好**， 先
`grep "ConvertToSysXxx" internal/system/model/*/conv_gen.go` 确认。缺了才补声明并
`go generate ./internal/system/model/...`。

---

## 2. BO / VO

### 查询 BO 必须单开一型

```go
// SysXxxQueryBo Xxx 列表查询条件（query 参数）。
//
// 与 SysXxxBo 分开而非复用：查询条件全部可选，而 SysXxxBo 的 binding:"required"
// 是新增场景的契约。Go 的 binding tag 没有校验分组概念，一个结构体只能有一套规则。
type SysXxxQueryBo struct {
	Name   string `form:"name"`
	Status string `form:"status"`
	// BeginTime/EndTime 创建时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
```

- 查询 BO 用 **`form`** tag（走 query），写入 BO 用 **`json`** tag（走 body）。
- 查询 BO **不加 `binding:"required"`**。
- 只放真正参与筛选的字段，不构造无效的查询能力。
- 入参字段偏离 Java 的写入 BO 时（如只收 2 个字段）， **另开一型**并在 doc 里写清为什么。

### VO 的 excel tag

```go
type SysXxxVo struct {
	XxxID  int64  `json:"xxxId" excel:"参数主键"`
	Name   string `json:"name" excel:"名称"`
	// 导出时按 excelDict 转标签，对齐 Java @ExcelDictFormat。
	Status string `json:"status" excel:"状态" excelDict:"0=正常,1=停用"`
}
```

- `excel` tag 的值 = Java `@ExcelProperty(value = ...)` 的原文， **逐字抄**。
- `@ExcelDictFormat(dictType = "sys_yes_no")` → `excelDict:"Y=是,N=否"`； 其它字典去 `script/sql/ry_vue.sql` 搜
  `sys_dict_data` 取 label/value。
- 没有 `excel` tag 的字段不导出（等价 Java `@ExcelIgnoreUnannotated`）。
- ID 字段 **保持裸 `int64`**，`pkg/excel` 自动按位数转字符串保精度。

---

## 3. Repository

**只有这一层碰 GORM。** 十个方法按需取用，签名照抄：

```go
var ErrXxxNotFound = errors.New("repository: Xxx 不存在")

type XxxRepository struct{ db *gorm.DB }

func NewXxxRepository(db *gorm.DB) *XxxRepository { return &XxxRepository{db: db} }

func (r *XxxRepository) SelectByID(ctx context.Context, id int64) (*model.SysXxx, error)
func (r *XxxRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*model.SysXxx, error)
func (r *XxxRepository) SelectPageList(ctx, q, page) (pkgrepo.PageResult[*model.SysXxx], error)
func (r *XxxRepository) SelectList(ctx, q, limit int) ([]*model.SysXxx, error)
func (r *XxxRepository) Insert(ctx context.Context, e *model.SysXxx) error
func (r *XxxRepository) UpdateByID(ctx, id int64, columns map[string]any) (int64, error)
func (r *XxxRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error)
func (r *XxxRepository) ExistsByYyy(ctx, val string, excludeID int64) (bool, error)
```

### 六条硬规则

**① 查询条件抽成独立函数，分页与导出共用。**

```go
// applyXxxQuery 应用查询条件。分页与导出两条路径必须共用它，否则过滤逻辑改一处漏一处。
func applyXxxQuery(db *gorm.DB, q bo.SysXxxQueryBo) *gorm.DB {
	if q.Name != "" {                                       // eqIfText/likeIfText：空串不筛
		db = db.Where("name LIKE ?", "%"+escapeLike(q.Name)+"%")
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 两端须同时给出（对齐 Java betweenParams 的 begin != null && end != null）：
	// 只给一端就筛会让前端清空半个日期框时结果突变。
	if q.BeginTime != "" && q.EndTime != "" {
		db = db.Where("create_time BETWEEN ? AND ?", q.BeginTime, q.EndTime)
	}
	return db
}
```

`escapeLike` 已在 `internal/system/repository/config_repository.go:144` 定义， **repository 同包共用，别重复定义**（同
`parseIDs` 的约束）。它把 LIKE 元字符转义成字面量——不转义的话搜 `%` 会命中全表、`_` 会变成任意单字符通配，这是与 Java
`likeIfText` 的有意差异；反斜杠排最前，避免重复转义。

**② `Model()` 不能省。** `del_flag` 过滤由 `LogicDelete` 挂在字段类型上，须先解析出实体 schema 才生效。
`Count`、`SelectList`、`SelectPageList` 全都要带。

**③ 默认排序只在调用方未指定时兜底。**

```go
// 不能无条件追加：GORM 合并排序子句时先注册的排在前，主键唯一会让后来的排序列失效。
if !page.HasOrder() {
	db = db.Order("xxx_id")
}
```

导出路径没有"调用方另指定排序"一说， **固定** `db.Order("xxx_id")`。

**④ 更新走 `map[string]any`，不走 struct。**
`Updates(struct)` 跳过零值，会让「清空备注/白名单」写不进库。

**⑤ 更新列里不带审计字段。** `update_by`/`update_time` 由 `pkg/repository` 的回调自动补。 插入同理，`create_*` 全由回调注入，
**不要手填**。

**⑥ 分页统一走 `pkgrepo.SelectPage`**，别手写 COUNT + LIMIT。 ctx 预先通过 `WithContext` 绑到 db 上，`SelectPage` 本身不收
ctx：

```go
db := applyXxxQuery(r.db.WithContext(ctx).Model(&model.SysXxx{}), q)
if !page.HasOrder() { db = db.Order("xxx_id") }
var rows []*model.SysXxx
return pkgrepo.SelectPage(db, page, &rows)
```

另一个变体 `SelectPageCtx(ctx, db, q, dest)` 等价，自动补 `WithContext`。

---

## 4. Service

```go
var ErrXxxNotFound  = errors.New("service: Xxx 不存在")
var ErrXxxKeyExists = errors.New("service: Xxx key 已存在")

type XxxService struct{}
var XxxSvcApp = new(XxxService)   // 包级实例，handler 直接用
```

- **每次调用现取 repo**：`repository.NewXxxRepository(database.DB())`，不缓存成字段。
- 返回 **VO**（`vo.Conv.ConvertToSysXxxVo`），不把 entity 漏给 handler。
- 分页要 **重建** `PageResult`：两个泛型实例是不同类型。
  `pkgrepo.Page(vo.Conv.ConvertToSysXxxVoList(res.Rows), res.Total)`
- 错误一律 `return err`， **由 handler 转 response**。需要自定义文案时用
  `errs.New(0, "文案", "")`（code 传 0 → 渲染成 500 + 该文案）。

### 插入：主键必须自己发号

```go
add := bo.Conv.ConvertToSysXxx(b)
add.XxxID = snowflake.Next()   // 各业务表主键均无 auto_increment
// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。
```

### 唯一性校验：`excludeID` 区分新增与修改

```go
// CheckXxxKeyUnique 校验 key 是否可用（对齐 Java 的「唯一即 true」）。
func (s *XxxService) CheckXxxKeyUnique(ctx, key string, excludeID int64) (bool, error)

s.CheckXxxKeyUnique(ctx, b.Key, 0)      // 新增：无自身可排除
s.CheckXxxKeyUnique(ctx, b.Key, b.ID)   // 修改：排除自身，改回原 key 不算冲突
```

### ⚠️ 存在性判定不要用受影响行数

```go
// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
// 那会把一次幂等的重复保存误报成"配置不存在"。
old, err := repo.SelectByID(ctx, b.ID)
if err != nil {
	if errors.Is(err, repository.ErrXxxNotFound) {
		return ErrXxxNotFound
	}
	return err
}
if _, err := repo.UpdateByID(ctx, b.ID, buildXxxUpdateColumns(b)); err != nil {
	return err
}
```

**例外**：`sys_client` 的 `UpdateClientStatus` 有意保留 `affected == 0 → ErrNotFound`， 对齐 Java `toAjax(0)`
的失败口径。改状态这类接口按 Java 原样即可。

### 更新列的取舍：可编辑字段一律写，控制字段缺省不改

```go
func buildXxxUpdateColumns(b *bo.SysXxxBo) map[string]any {
	columns := map[string]any{
		"name": b.Name,
		// 一律写入，让前端能把备注清空——这正是编辑表单的本意。
		"remark": b.Remark,
	}
	// 缺省即视为不改：漏传字段不该把线上的 'Y' 刷成空串，
	// 那会让内置数据失去删除保护。等效于 Java updateById 对 null 字段的跳过。
	if b.ConfigType != "" {
		columns["config_type"] = b.ConfigType
	}
	return columns
}
```

判断依据： **该字段被清空是用户的合法意图吗？** 是 → 一律写（remark、白名单）； 否 → 空值跳过（status、timeout、内置标记）。

### 删除：先整批校验，再删

```go
rows, err := repo.SelectByIDs(ctx, ids)
// 先整批校验再删，不边删边校验：Java 侧靠抛异常回滚事务，这里没有事务包裹，
// 一旦先删了几行再撞上内置参数就会留下删一半的状态。
for _, e := range rows {
	if e.ConfigType == constant.Yes {
		return errs.New(0, fmt.Sprintf("内置参数【%s】不能删除", e.ConfigKey), "")
	}
}
if _, err := repo.DeleteByIDs(ctx, ids); err != nil { return err }
// 删除后再失效：提前清缓存会让删除失败时白丢一批热数据。
for _, e := range rows { _ = cache.Evict(ctx, constant.CacheSysXxx, e.Key) }
```

### 缓存：Java 注解 → Go 调用

| Java 注解                      | Go 写法                                              |
|--------------------------------|------------------------------------------------------|
| `@Cacheable(key="#k")`         | 读头 `cache.Get` 命中即返回；miss 查库后 `cache.Put` |
| `@CachePut(key="#k")`          | 写库**成功后** `cache.Put(ctx, group, k, val, ttl)`  |
| `@CacheEvict(key="#k")`        | `cache.Evict(ctx, group, k)`                         |
| `@CacheEvict(allEntries=true)` | `cache.EvictGroup(ctx, group)`                       |
| 防击穿的读穿                   | `cache.GetOrSet(...)`（内含 singleflight）           |

- 组名与 TTL 加在 `pkg/constant/cache_names.go`。 TTL 取值看 Java `CacheNames` 的 `#ttl` 后缀：`sys_client#30d` →
  `30*24*time.Hour`， **无后缀 → `0`（不过期）**。
- 缓存写/删（`Put`/`Evict`/`EvictGroup`） **一律 `_ =` 忽略错误**（fail-open，Redis 挂了要能继续走 DB）。 但 `Get` 返回两个值，
  **不能** `_ = cache.Get(...)`——用 `hit, _ := cache.Get(ctx, group, key, &dest)`。
- 改了 key 的更新， **旧 key 要单独 Evict**——`@CachePut` 只写新 key，旧的会成孤儿。
- Java `@Cacheable` 缓存的是返回值， **"查不到"的空串也会入缓存**，照搬即可（写路径的 `@CachePut` 会覆盖）。

---

## 5. Handler

固定骨架，`ClientApi`/`ConfigApi` 逐字同构：

```go
type XxxApi struct{}
var XxxApiApp = new(XxxApi)
```

### 分页查询：绑两次

```go
func (a *XxxApi) List(c *gin.Context) {
	var q bo.SysXxxQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}
	res, err := systemservice.XxxSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil { _ = c.Error(err); return }
	c.JSON(http.StatusOK, response.Ok(res))
}
```

筛选条件与分页参数同在 query 上， **分两次绑定同一份 URL 参数**——query 绑定不消费 body，可重复调用。

### 导出：POST + `ShouldBind` + 多取一行

```go
func (a *XxxApi) Export(c *gin.Context) {
	var q bo.SysXxxQueryBo
	if err := c.ShouldBind(&q); err != nil {   // 同时吃 form body 与 query
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	rows, err := systemservice.XxxSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil { _ = c.Error(err); return }
	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "Xxx数据"); err != nil { _ = c.Error(err); return }
}
```

- 走 **POST**（前端 `commonExport` 以 form 表单提交），不是 GET。
- `excel.MaxRows+1` 多取一行判超限，避免"先捞完百万行再拒绝"。
- **业务 handler 中唯一不返回 `response.R` 的接口**，响应体是二进制附件 （探针 `/ping` 是路由内联的 `gin.H` 例外，不算业务接口）。
- sheetName = Java `.sheetName("...")` 的原文。

### 主键入参

```go
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
id, err := strconv.ParseInt(c.Param("xxxId"), 10, 64)
if err != nil || id <= 0 {
	_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("xxxId")))
	return
}

ids, err := parseIDs(c.Param("xxxIds"))   // 批量：复用 handler 包内已有的 parseIDs
```

`parseIDs` 已在 `internal/system/handler/client_handler.go:185` 定义（ **同包共用，别重复定义**）：
逗号分隔，任一段非法即整体拒绝——静默丢弃会删成部分成功。 其他模块的 handler 包（如 `internal/auth/handler`） **没有**
这个函数，需要时在各自包内补一个。

### 写接口：绑 JSON + 翻译哨兵错误

```go
func (a *XxxApi) Edit(c *gin.Context) {
	var b bo.SysXxxBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysXxxBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.XxxID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", ""))
		return
	}
	if err := systemservice.XxxSvcApp.UpdateXxx(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrXxxKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改'%s'失败，key已存在", b.Name), ""))
			return
		}
		if errors.Is(err, systemservice.ErrXxxNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "Xxx 不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
```

### 三条铁律

1. **错误一律 `_ = c.Error(...)` 后 `return`**，绝不自己 `c.JSON` 错误响应—— 统一由 `middleware.Recover` 渲染（HTTP 状态恒
   200）。
2. 只翻译 service 导出的 **哨兵错误**（`errors.Is`），其余原样 `c.Error(err)` 兜底。
3. 出参只用 `response.Ok(...)` / `response.OkVoid()`， **不写 `gin.H`**。

### 错误码选用

响应体字段 `code`（ **HTTP 状态恒为 200**，下面列的是 body 里的 `code` 值，不是 HTTP 状态码）：

| 场景                   | 构造                                                             | 响应体 code  |
|------------------------|------------------------------------------------------------------|--------------|
| 参数格式/缺失          | `errs.New(response.CodeBadRequest, "参数校验失败", err.Error())` | 400          |
| 业务失败（重复、冲突） | `errs.New(response.CodeFail, "xx失败，yy已存在", "")`            | 500          |
| 资源不存在             | `errs.New(response.CodeNotFound, "Xxx 不存在", "")`              | 404          |
| service 内自定义文案   | `errs.New(0, "内置参数【k】不能删除", "")`                       | 500 + 该文案 |

第三个参数 `Detail` **只进日志不回前端**，放 `err.Error()` 这类内部细节。

---

## 6. Router：注册顺序有讲究

```go
// xxxLogTitle 操作日志模块名，对照 Java @Log(title = "...")。
const xxxLogTitle = "参数管理"

xxx := protected.Group("/xxx")
xxx.GET("/list", satoken.CheckPermission("system:xxx:list"), handler.XxxApiApp.List)
xxx.GET("/configKey/:configKey", sagin.CheckLogin(), handler.XxxApiApp.GetByKey)
xxx.GET("/:xxxId", satoken.CheckPermission("system:xxx:query"), handler.XxxApiApp.GetInfo)
xxx.POST("/export", satoken.CheckPermission("system:xxx:export"),
	oplog.Log(xxxLogTitle, enum.BusinessTypeExport), handler.XxxApiApp.Export)
xxx.POST("", satoken.CheckPermission("system:xxx:add"),
	oplog.Log(xxxLogTitle, enum.BusinessTypeInsert),
	repeatsubmit.RepeatSubmit(0, ""), handler.XxxApiApp.Add)
xxx.PUT("", satoken.CheckPermission("system:xxx:edit"),
	oplog.Log(xxxLogTitle, enum.BusinessTypeUpdate),
	repeatsubmit.RepeatSubmit(0, ""), handler.XxxApiApp.Edit)
xxx.DELETE("/refreshCache", satoken.CheckPermission("system:xxx:remove"),
	oplog.Log(xxxLogTitle, enum.BusinessTypeClean), handler.XxxApiApp.RefreshCache)
xxx.DELETE("/:xxxIds", satoken.CheckPermission("system:xxx:remove"),
	oplog.Log(xxxLogTitle, enum.BusinessTypeDelete), handler.XxxApiApp.Remove)
```

### 中间件顺序：鉴权 → 日志 → 防重 → handler

**顺序不是风格问题，改了行为就变**：

- **鉴权最前**：未授权请求不该白占一个防重锁。
- **日志在防重之前**：被防重挡掉的请求 handler 没执行， 与 Java 侧 `RepeatSubmitAspect` 抛异常后 `LogAspect` 记一条失败日志一致。
- **`repeatsubmit` 须在 `encrypt.ApiEncrypt()` 之后**：指纹要用解密后的明文， 否则密文每次随机密钥、同样入参算出不同指纹，防重直接失效。

### 路径细节

- 根路径用 **`""` 而非 `"/"`**，后者会注册成 `/xxx/`。
- 静态段（`/export`、`/refreshCache`、`/updateByKey`）与同层通配段（`/:id`） **可以共存**， gin 静态段优先，无需刻意调整注册顺序。但
  **值得写测试钉住**—— 这条规则一旦变化，`DELETE /config/refreshCache` 会被当成"删除主键为 refreshCache 的配置"而
  **静默走错分支**。

### 注解速查

| Java                                          | Go                                                                         | 说明                                                                                                                             |
|-----------------------------------------------|----------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------|
| `@SaCheckPermission("system:xxx:list")`       | `satoken.CheckPermission("system:xxx:list")`                               | 多个权限码是 **OR**，语义是"任一命中放行"                                                                                        |
| `@SaCheckRole("admin")`                       | `satoken.CheckRole("admin")`                                               | 同上                                                                                                                             |
| 仅需登录（无权限注解）                        | `sagin.CheckLogin()`                                                       | Java 没挂 `@SaCheckPermission` 的接口用这个                                                                                      |
| 完全公开                                      | `sagin.Ignore()`                                                           | **仅在路由位于带 `plugin.TokenInterceptor()` 的组内时**才需要；注册在 `protected` 组之外（如 `/ping`）的公开路由什么标记都不用加 |
| `@Log(title="x", businessType=INSERT)`        | `oplog.Log("x", enum.BusinessTypeInsert)`                                  |                                                                                                                                  |
| `@RepeatSubmit()`                             | `repeatsubmit.RepeatSubmit(0, "")`                                         | `0` = 默认 5s；**< 1s 注册期 panic**                                                                                             |
| `@RepeatSubmit(interval=10000)`               | `repeatsubmit.RepeatSubmit(10*time.Second, "")`                            |                                                                                                                                  |
| `@RateLimiter(time=60,count=10,limitType=IP)` | `ratelimiter.RateLimiter(time.Minute, 10, ratelimiter.LimitTypeIP, 0, "")` |                                                                                                                                  |
| `@ApiEncrypt`                                 | `encrypt.ApiEncrypt()`                                                     |                                                                                                                                  |

`BusinessType` 取值即 Java `ordinal()`（库里存数字）：
`Other=0, Insert=1, Update=2, Delete=3, Grant=4, Export=5, Import=6, Force=7, GenCode=8, Clean=9`。
**顺序不可调整、不可插入新值，只能末尾追加。**

`oplog.Log` 可选项：`WithoutRequestData()`、`WithoutResponseData()`、
`WithExcludeParams("field")`、`WithOperatorType(enum.OperatorTypeMobile)`。 密码类字段已由 `constant.ExcludeProperties`
全局排除，无需重复声明。

### 新增进程入口的必备初始化

顺序有依赖，照抄 `cmd/standalone/main.go`：

```go
config.Load(...)
jsonx.Init()        // 必须在首个 c.JSON / 参数绑定之前接管 gin codec
database.Init(); redis.Init(); satoken.Init(); encrypt.Init()
snowflake.Init()    // 主键发号器，插入前必须就绪
captcha.Init(); ratelimiter.Init(); repeatsubmit.Init()   // 三者都依赖 redis
oplog.Init(systemservice.OperLogSvcApp.RecordOper)        // 依赖 database + snowflake
```

全局中间件：`Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS → I18n`。

---

## 7. 测试：只测判断，不测搬运

用 **DryRun GORM**（拼 SQL 不连库）测 SQL 组装，纯函数直接测。
`dryClientDB(t)` 已在 `repository` 包内，直接用。

值得写的：

- 查询条件全传时都落到 WHERE、空串不落（`likeIfText` 语义）
- 时间区间只给一端时不筛
- LIKE 元字符转义
- `buildXxxUpdateColumns` 的写/跳过取舍
- 真实 `RegisterRoutes` 的路由表形状（`r.Routes()` 断言，别另建探针）

不值得写的：getter、单纯的 VO 字段拷贝、`response.Ok` 包装。

`dryClientDB(t)` 定义在 `internal/system/repository/client_repository_test.go`（ **test-only**，跨包取不到）。 新测试文件须放在
**同目录**且声明 **`package repository`**（非 `package repository_test`）才能用它；
`pkg/repository` 里的同名辅助同理，只在它自己包内的测试可用。若需要在 service 层做 DB 测试， 得在 service 包的自测里另起一个同形
helper，别指望跨包复用。

路由测试需要 sa-token Manager，装内存态即可（不依赖 Redis）：

```go
sagin.SetManager(sagin.NewBuilder().Storage(memory.NewStorage()).Build())
```

---

## 8. 提交前自检

```bash
go build ./...    # 必过
go vet ./...      # 必过
go test ./...     # 新增测试必过
go mod tidy       # 引入新依赖后
```

> `pkg/config` 的 `TestRealYAMLMatchesDefaults` **当前是已知失败**
> （application.yaml 的 captcha.enable 与默认值不一致），与业务改动无关，不必修。
> 但 **别把它当成"测试本来就红"的挡箭牌**——除它以外任何失败都要处理。

### 清单

- [ ] 分层单向：handler → service → repository，无反向依赖、无 handler 直连 repo
- [ ] 只有 repository 出现 `gorm`
- [ ] handler 出参只有 `response.R` / `PageResult`（导出接口除外）
- [ ] 错误全走 `c.Error` + `middleware.Recover`，没有手写错误 JSON
- [ ] ID 字段是裸 `int64`，没加 `,string`
- [ ] 插入前 `snowflake.Next()`，没手填审计字段
- [ ] 更新走 `map`，可编辑字段一律写、控制字段空值跳过
- [ ] 存在性判定没依赖受影响行数（改状态类接口除外）
- [ ] 缓存 key/TTL 与 Java `CacheNames` 对齐，改 key 的更新清了旧 key
- [ ] 路由：鉴权 → 日志 → 防重，根路径用 `""`
- [ ] 注释只写"为什么"，没有"对照 Java XxxAspect.doBefore"这类方法级映射流水账

---

## 9. 与 Java 的既有差异（照做，别"修正"）

这些是有意为之，不是 bug：

| 点                   | 本项目                              | Java                                                    |
|----------------------|-------------------------------------|---------------------------------------------------------|
| LIKE 元字符          | 转义，按字面量匹配                  | 不转义，搜 `%` 会命中全表                               |
| 导出字典未命中       | 原样返回原值                        | 留空                                                    |
| 权限码多值           | OR                                  | `@SaCheckPermission` 默认 AND（上游全是单值，不可观测） |
| 防重指纹             | 请求体 + query 串，sha256           | 方法入参，md5                                           |
| `interval < 1s`      | 注册期 panic                        | 运行期抛异常                                            |
| oplog 的 `Method`    | gin HandlerName                     | 类名.方法名                                             |
| oplog 入参采集       | handler **之前**（body 是一次性流） | 切面之后                                                |
| 分页 `pageSize` 上限 | 500                                 | `Integer.MAX_VALUE`                                     |
