# pkg/middleware

全局 Gin 中间件，对应原项目散落在 `ruoyi-common-web` / `ruoyi-common-security` / `ruoyi-common-satoken` 等模块里的
Servlet Filter、Spring Interceptor 和 `@RestControllerAdvice`。

## 一句话说清 Java 侧的结构

Java 的横切逻辑 **没有集中在一个包**，而是分散在各 `ruoyi-common-*` 模块中，靠 Spring Boot 的
`META-INF/spring/org.springframework.boot.autoconfigure.AutoConfiguration.imports` 自动装配（ **不是** `@ComponentScan`）。
找一个中间件的注册处时，先看这四个清单文件：

| 模块                    | 注册的配置类                                                        |
|-------------------------|---------------------------------------------------------------------|
| `ruoyi-common-web`      | `CaptchaConfig` / `FilterConfig` / `I18nConfig` / `ResourcesConfig` |
| `ruoyi-common-security` | `AllUrlHandler` / `SecurityConfig`                                  |
| `ruoyi-common-encrypt`  | `ApiDecryptAutoConfiguration`                                       |
| `ruoyi-common-log`      | `LogAspect`                                                         |

Go 侧全部收拢到本包，由各模块 `router.go` 显式 `r.Use(...)` 注册 —— 顺序看得见，比 Spring 的 `@Order` 数值好读。

## 全局逐请求关卡（阶段 0 范围）

路径以 `ruoyi-common/` 为根，省略 `src/main/java/org/dromara/common/`。

| # | 关注点          | 原 Java 位置                                                                            | 本包文件            |
|---|-----------------|-----------------------------------------------------------------------------------------|---------------------|
| 1 | 全局异常        | `ruoyi-common-web/…/web/handler/GlobalExceptionHandler.java` + 另 5 个 advice（见下）   | ✅ `recover.go`     |
| 2 | CORS            | `ruoyi-common-web/…/web/config/ResourcesConfig.java:73-86`（`CorsFilter` bean）         | `cors.go`           |
| 3 | TraceID         | **原项目不存在，净新增**                                                                | `trace.go`          |
| 4 | 可重复读 Body   | `ruoyi-common-web/…/web/filter/RepeatableFilter.java` + `RepeatedlyRequestWrapper.java` | `body.go`           |
| 5 | 请求日志 + 耗时 | `ruoyi-common-web/…/web/interceptor/PlusWebInvokeTimeInterceptor.java`                  | `logger.go`         |
| 6 | XSS 过滤        | `ruoyi-common-web/…/web/filter/XssFilter.java` + `XssHttpServletRequestWrapper.java`    | `xss.go`            |
| 7 | i18n            | `ruoyi-common-web/…/web/config/I18nConfig.java` + `web/core/I18nLocaleResolver.java`    | `i18n.go`           |
| 8 | 鉴权            | `ruoyi-common-security/…/security/config/SecurityConfig.java:80-119`                    | `auth.go`（阶段 1） |
| 9 | 响应增强        | `ruoyi-common-web/…/web/advice/ResponseEnhancementAdvice.java`                          | 阶段 3 再建         |

### 注册顺序

```
Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS → I18n → Auth
```

两个顺序约束不能动：

- **CORS 必须在 Auth 之前**，否则浏览器 preflight（`OPTIONS`，不带 token）会被 401，前端拿不到跨域头。
- **RepeatableBody 必须在 AccessLog 之前**，Java 侧 `PlusWebInvokeTimeInterceptor` 只在请求是
  `RepeatedlyRequestWrapper` 时才读 body（源码里显式判类型），Go 里同理：body 是一次性 `io.ReadCloser`，日志读完 handler
  就绑不到参数了。

Go 实现即 `io.ReadAll` 后 `c.Request.Body = io.NopCloser(bytes.NewReader(b))`。除了日志，阶段 3 的 `@Log` 操作日志也依赖它。

各进程入口用 `gin.New()` **而非 `gin.Default()`**：后者自带 `gin.Recovery()`，只写 500 空响应，与 `Recover()`
职责重叠且不输出 `response.R`。`Recover()` 挂在最外层，才能兜住后续中间件自身的 panic。

## 逐项要注意的地方

### 1. 全局异常：6 个 advice 合成 1 个中间件

Java 有 6 个 `@RestControllerAdvice`，Spring 合并成一条 advice 链。Go 里合成 **一个** Recover 中间件：

| 文件                                                                        | 负责                                                                          |
|-----------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| `ruoyi-common-web/…/web/handler/GlobalExceptionHandler.java`                | 主体，18 个 `@ExceptionHandler` → `R<Void>`                                   |
| `ruoyi-common-satoken/…/satoken/handler/SaTokenExceptionHandler.java`       | `NotPermission`/`NotRole`→403，`NotLogin`→401，即把鉴权中间件抛的异常转成响应 |
| `ruoyi-common-mybatis/…/mybatis/handler/MybatisExceptionHandler.java`       | `DuplicateKeyException` 等转友好文案                                          |
| `ruoyi-common-redis/…/redis/handler/RedisExceptionHandler.java`             | lock4j `LockFailureException`                                                 |
| `ruoyi-common-sms/…/sms/handler/SmsExceptionHandler.java`                   | `SmsBlendException`                                                           |
| `ruoyi-modules/ruoyi-workflow/…/workflow/handler/FlowExceptionHandler.java` | warm-flow `FlowException`（阶段 5）                                           |

主 handler 里值得对齐的行为：

- `RuntimeException` / `Exception` 兜底时会生成 **随机 8 位 `errorId`** 拼进返回 message，日志里同时打这个 id ——
  这是原项目唯一的请求关联手段（因为没有 traceId）。Go 有 TraceID 后可以直接用 traceId 替代，但要意识到这是 **行为变更**。
- `handleIoException` 会读 yaml 的 `message.path`（`/resource/message`），对 SSE 断连的 IO 异常 **静默不打日志**。
- `AsyncRequestTimeoutException` 是 no-op，不返回任何东西。

#### Go 实现的有意偏差（`recover.go`）

| 位置         | 偏差                                 | 原因                                                                                        |
|--------------|--------------------------------------|---------------------------------------------------------------------------------------------|
| panic 兜底   | Java 无对等物                        | Spring 靠异常冒泡到 advice；Go 不拦 panic 会**打挂整个进程**，而非只失败一个请求            |
| 兜底文案     | Java 分「未知异常」/「系统异常」两句 | Java 能按 `RuntimeException` vs `Exception` 区分，Go 的 panic 值无此层次，统一一句          |
| SSE 断连判定 | Java 用 URI 前缀匹配 `message.path`  | Go 直接判 `*net.OpError` + broken pipe/connection reset，比配路径准，也不必等 SSE 模块迁完  |
| 参数校验异常 | Java 有 5 个 validation handler      | Go 的 binding 错误由 handler 就地转 `errs.New`，不在中间件里拆 `validator.ValidationErrors` |
| 405 / 404    | Java 有专门 handler                  | Gin 用 `NoRoute` / `NoMethod` 注册，不走错误链；待 `auth.go` 一并处理（见下方鉴权的坑）     |

两条硬约束，改动时注意别破：

1. **HTTP 状态码恒为 200**，业务码只放响应体 —— Java 侧这些 advice 返回 `R<Void>` 且未标 `@ResponseStatus`， 前端拦截器只认
   `body.code`，改成真实 4xx/5xx 会让前端失效。唯一例外是 Java 的 `SseException`（真 401），SSE 迁移时再处理。
2. **非业务错误必须脱敏**。原始 `error` 里可能有 SQL 片段、内网地址、文件路径，只回「兜底文案 + 8 位错误编号」， 细节进日志。
   `ServiceError.Detail` 同理，只打日志不回前端。

错误来源有两条，都要覆盖：`panic`（bug）和 `c.Error(err)`（handler 主动登记，等价 Java 的 `throw`）。 业务错误应走 `c.Error`，
**不要 panic** —— panic 一律当系统异常处理，不解析业务码。

### 2. CORS：yaml 里查不到配置，全是代码默认值

配置类 `ruoyi-common-web/…/web/config/properties/CorsProperties.java`，前缀 `web.cors`。 **但 `application.yml` 及
dev/prod 里都没有这个 key**，实际生效的是代码默认值：

```
allowCredentials     = true
allowedOriginPatterns= ["*"]
allowedHeaders       = ["*"]
allowedMethods       = ["*"]
maxAge               = 1800
```

> `allowCredentials=true` 配 `Origin: *` 在浏览器端是 **非法组合**，会被拒。Java 侧用的是
> `allowedOriginPatterns`（Spring 特有，会把 `*` 回显成具体 Origin）而非 `allowedOrigins`。Go 里手写 CORS
> 时必须 **回显请求的 Origin**，不能直接吐 `*`，否则带 cookie/凭证的请求全挂。

另有一个 WebSocket 专用开关 `message.allowedOrigins: '*'`（`application.yml:225`），与 HTTP CORS 无关。

### 3. TraceID：原项目没有，别去找

全项目零 `MDC.put`、零 `TransmittableThreadLocal`、零 Micrometer Tracing / Sleuth。
`ruoyi-admin/src/main/resources/logback-plus.xml:104,114` 有 `%tid` 但 **被注释掉了**（残留的 SkyWalking/TLog 配置）。 生效的
pattern 只有 `%d{...} [%thread] %-5level %logger{36} - %msg%n`。

所以这块 **没有对照物**，是 Go 侧自主设计。建议：请求头有 `X-Request-Id` 就沿用，否则生成；存 `context`，回写响应头，并让日志中间件和
Recover 都带上它。

### 4. 请求日志：脱敏和截断都要照做

`PlusWebInvokeTimeInterceptor` 的三个细节：

- `preHandle` 打 `[PLUS]开始请求 => URL[...],参数...`，`afterCompletion` 打 `[PLUS]结束请求 => URL[...],耗时:[N]毫秒`， 用
  `ThreadLocal<StopWatch>` 计时（Go 直接在中间件闭包里存 `time.Now()`，比 ThreadLocal 简单）。
- **脱敏**：JSON body 和 query 参数都会剔除 `SystemConstants.EXCLUDE_PROPERTIES`。 本项目已移植为
  `constant.ExcludeProperties`（`pkg/constant/system.go:21`），直接用，别另写一份。
- **截断**：参数日志最长 4000 字符。

### 5. XSS：开关 + 两级跳过

注册在 `ruoyi-common-web/…/web/config/FilterConfig.java:29-39`，`@ConditionalOnProperty` 挂 `xss.enabled`。

- **跳过 GET / DELETE**（filter 内部判 method 直接放行）
- **跳过 `xss.excludeUrls`**：现为 `/system/notice`、`/warm-flow/save-json`（富文本/JSON 存原文，过滤会破坏内容）
- 配置类 `web/config/properties/XssProperties.java`，yaml 在 `application.yml:190-196`

### 6. i18n：从 `content-language` 取，不是 `Accept-Language`

`web/core/I18nLocaleResolver.java` 读的是 **`content-language` 请求头**（非标准用法，但要对齐前端）， 下划线归一成横线，取不到回落
`Locale.getDefault()`。`setLocale` 是刻意的空实现。 词条目录由 `spring.messages.basename: i18n/messages` 指定。

### 7. 鉴权（阶段 1）：四步校验 + 一个易踩的坑

`SecurityConfig.java:80-119` 注册 sa-token `SaInterceptor` 到 `/**`，排除 `securityProperties.getExcludes()`。每请求做四件事：

1. `StpUtil.checkLogin()` 校验 token
2. header / query 的 `clientid` 与 token extra 里的 clientid 交叉比对，不符抛 `NotLoginException` code `-100`，
   message「客户端ID与Token不匹配」
3. `validateClientAccessRules()`（`:147-166`）按客户端做 **URL 路径白名单**，不符抛
   `NotPermissionException("当前客户端未授权访问该接口路径")`
4. 同方法内按客户端做 **IP 白名单**（`NetUtils.isMatchIpRule`），不符抛「当前客户端IP不在白名单内」

> **坑**：它用 `SaRouter.match(allUrlHandler.getUrls())` 匹配。
> `ruoyi-common-security/…/security/handler/AllUrlHandler.java`
> 在启动时遍历 `RequestMappingHandlerMapping` 收集 **所有已注册路由**（`{pathVariable}` 替换成 `*`）。
> 也就是说 **未注册的路径根本不进鉴权，直接落 404 而非 401**。Gin 的 `NoRoute` 默认行为不同，
> 若前端/测试依赖「乱路径返回 404 不是 401」，需显式对齐。

免鉴权名单：yaml `security.excludes`（`ruoyi-admin/src/main/resources/application.yml:100-113`），当前含
`/*.html`、`/**/*.html`、`/**/*.css`、`/**/*.js`、`/favicon.ico`、`/error`、`/*/api-docs`、`/*/api-docs/**` 等。

token 解析规则在 `ruoyi-common-satoken/src/main/resources/common-satoken.yml`（由 `SaTokenConfig` 用
`@PropertySource` 加载），注意 `is-read-cookie: false` —— **刻意关掉 cookie 认证来消除 CSRF**，Go 侧同样只从 header 取，
不要为了方便加 cookie 回落。

### 9. 响应增强：阶段 3 再说

`web/advice/ResponseEnhancementAdvice.java` 是 `ResponseBodyAdvice<Object>`，每个 JSON 响应过一遍
`JsonValueEnhancer.enhance(body)`，是字典翻译 / `@Translation` 字段填充的钩子。 阶段 0 **不建空壳**，等阶段 3 字典管理落地时按需引入。

## 不属于本包的（注解驱动，非全局）

这些在 Java 里是 `@Aspect` 或按注解生效，Go 侧应做成 **按路由挂的中间件或 handler 内显式调用**，不要塞进全局链：

| 关注点            | 原 Java 位置                                                                               | 阶段     |
|-------------------|--------------------------------------------------------------------------------------------|----------|
| `@Log` 操作日志   | `ruoyi-common-log/…/log/aspect/LogAspect.java`（发 `OperLogEvent` 异步落库）               | 阶段 3   |
| 登录日志          | **不是中间件**，`ruoyi-admin/…/web/service/SysLoginService.java` 里手动发 `LoginInfoEvent` | 阶段 1/3 |
| `@RateLimiter`    | `ruoyi-common-redis/…/redis/aspectj/RateLimiterAspect.java`                                | 阶段 4+  |
| `@RepeatSubmit`   | `ruoyi-common-redis/…/redis/aspectj/RepeatSubmitAspect.java`                               | 阶段 4+  |
| `@Lock4j`         | `ruoyi-common-redis/…/redis/config/Lock4jConfig.java`                                      | 阶段 4+  |
| `@ApiEncrypt`     | `ruoyi-common-encrypt/…/encrypt/filter/CryptoFilter.java`                                  | 按需     |
| `@DataPermission` | `ruoyi-common-mybatis/…/mybatis/interceptor/PlusDataPermissionInterceptor.java`            | 阶段 4.1 |

`CryptoFilter` 有点特殊：它 **注册成全局 filter**，但内部自己通过 `RequestMappingHandlerMapping` 查
`@ApiEncrypt` 注解，只对标注的方法真正生效。Go 里没必要照搬这个结构，按路由挂即可。

数据权限见 `MIGRATION.md` 阶段 4.1 —— Java 是 MyBatis 拦截器改写 SQL，Go 要在 repository 层用 GORM Scopes 手写。 本包只负责
**把当前用户的数据范围写进 context**，SQL 条件由 repository 层拼。

## 相关 yaml key 速查

除 sa-token 那份外，均在 `ruoyi-admin/src/main/resources/application.yml`。已确认 dev/prod 两个 profile **没有覆盖任何中间件相关
key**，看主文件就够。

| key                        | 行号                                                         | 作用                                        |
|----------------------------|--------------------------------------------------------------|---------------------------------------------|
| `security.excludes`        | 100-113                                                      | 鉴权免登名单                                |
| `xss.enabled`              | 190-192                                                      | XSS 过滤开关                                |
| `xss.excludeUrls`          | 193-196                                                      | XSS 跳过路径                                |
| `web.cors.*`               | **缺失**                                                     | CORS，走代码默认值                          |
| `spring.messages.basename` | 61-63                                                        | i18n 词条目录                               |
| `message.path`             | 223                                                          | 被 `handleIoException` 读来静默 SSE 断连    |
| `api-decrypt.*`            | 148-158                                                      | `CryptoFilter` 开关与 RSA 密钥              |
| `sa-token.token-name`      | 91                                                           | token 的 header/param 名（`Authorization`） |
| `sa-token.jwt-secret-key`  | 97                                                           | JWT 签名密钥                                |
| `is-read-cookie: false`    | `ruoyi-common-satoken/src/main/resources/common-satoken.yml` | 关掉 cookie 认证                            |
