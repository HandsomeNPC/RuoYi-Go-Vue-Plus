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

Go 侧全部收拢到本包，由 `Register(r)` 按固定顺序显式 `r.Use(...)` —— 顺序看得见，比 Spring 的 `@Order` 数值好读。

## 全局逐请求关卡（阶段 0 范围）

路径以 `ruoyi-common/` 为根，省略 `src/main/java/org/dromara/common/`。

| #  | 关注点          | 原 Java 位置                                                                            | 本包文件        |
|----|-----------------|-----------------------------------------------------------------------------------------|-----------------|
| 1  | 全局异常        | `ruoyi-common-web/…/web/handler/GlobalExceptionHandler.java` + 另 5 个 advice（见下）   | ✅ `recover.go` |
| 2  | CORS            | `ruoyi-common-web/…/web/config/ResourcesConfig.java:73-86`（`CorsFilter` bean）         | ✅ `cors.go`    |
| 3  | TraceID         | **原项目不存在，净新增**                                                                | ✅ `trace.go`   |
| 4  | 接口加解密      | `ruoyi-common-encrypt/…/encrypt/filter/CryptoFilter.java` + 另 2 个 wrapper             | ✅ `crypto.go`  |
| 5  | 可重复读 Body   | `ruoyi-common-web/…/web/filter/RepeatableFilter.java` + `RepeatedlyRequestWrapper.java` | ✅ `body.go`    |
| 6  | 请求日志 + 耗时 | `ruoyi-common-web/…/web/interceptor/PlusWebInvokeTimeInterceptor.java`                  | ✅ `logger.go`  |
| 7  | XSS 过滤        | `ruoyi-common-web/…/web/filter/XssFilter.java` + `XssHttpServletRequestWrapper.java`    | ✅ `xss.go`     |
| 8  | i18n            | `ruoyi-common-web/…/web/config/I18nConfig.java` + `web/core/I18nLocaleResolver.java`    | ✅ `i18n.go`    |
| 9  | 鉴权            | `ruoyi-common-security/…/security/config/SecurityConfig.java:80-119`                    | ✅ `auth.go`    |
| 10 | 响应增强        | `ruoyi-common-web/…/web/advice/ResponseEnhancementAdvice.java`                          | 阶段 3 再建     |

另有三个没有 Java 直接对照物的文件：`register.go`（按序注册全部中间件，对应 Spring 的四个 `AutoConfiguration.imports`
清单）、`path.go`（Ant 路径匹配，对应 `AntPathMatcher`，被 `xss.go`、`crypto.go`、`auth.go` 共用）与
`ip.go`（客户端 IP 提取与 IP 规则匹配，对应 `ServletUtils.getClientIP` + `NetUtils.isMatchIpRule`， 被 `auth.go` 的客户端
IP 白名单用，阶段 4+ 的 `@RateLimiter` 会复用）。 加解密原语不在本包，在
`pkg/encrypt`（对应 `encrypt/utils/EncryptUtils.java`）—— 阶段 4+ 的
`@EncryptField` 字段级加密会复用它，那条路径与 HTTP 无关、不该 import gin。 登录态原语（JWT 签发/校验、Redis 会话、密码哈希）在
`pkg/auth`，同样不 import gin —— `internal/*/service` 要签发与销毁会话。 配置结构体不在本包，在
`pkg/config/middleware.go`（见下方「配置怎么读到的」）。

### 注册顺序

```
Recover → CORS → TraceID → ApiEncrypt → RepeatableBody → AccessLog → XSS → I18n → Auth
```

四个顺序约束不能动：

- **CORS 必须在 Auth 之前**，否则浏览器 preflight（`OPTIONS`，不带 token）会被 401，前端拿不到跨域头。
- **ApiEncrypt 必须在 RepeatableBody 之前**，否则 `AccessLog` 只能看到 base64 密文、脱敏形同虚设，且 body 已被读走、handler
  绑不到参数。完整依据见下方「加解密的顺序依据」。
- **RepeatableBody 必须在 AccessLog 之前**，Java 侧 `PlusWebInvokeTimeInterceptor` 只在请求是
  `RepeatedlyRequestWrapper` 时才读 body（源码里显式判类型），Go 里同理：body 是一次性 `io.ReadCloser`，日志读完 handler
  就绑不到参数了。
- **XSS 必须在所有「读 gin 参数」的中间件之前**（同时也必须在 `RepeatableBody` 之后）。gin 的 `c.Query` 会把
  `URL.Query()` 的结果缓存进内部 `queryCache`，缓存一旦建立， XSS 再改 `URL.RawQuery` 就 **静默失效**。当前链路里 XSS
  之前没有任何一环读 gin 参数（`AccessLog` 走 `c.Request.ParseForm`，不碰 gin 的缓存；`I18n` 排在 XSS 之后且只读请求头）， 阶段
  1 的 `Auth` 读 `clientid` 排在 XSS 之后 —— 拿到的是清洗后的值。 这条约束由 `TestXSSIneffectiveIfQueryReadEarlier` 锁住：它
  **故意**把读参数的中间件
  前置并断言清洗失效，将来谁把鉴权挪到 XSS 前面，那条用例就会以「清洗突然生效了」的形式报错。

第五条约束来自 i18n，宽松得多： **I18n 必须在 Auth 之前**（鉴权的提示文案要走词条）。它不读 body、不改请求， 与前四条没有交集，详见下方
i18n 一节。

> 截至当前，`Recover → CORS → TraceID → ApiEncrypt → RepeatableBody → AccessLog → XSS → I18n → Auth`
> **已全部**在 `register.go` 的 `Register(r)` 里注册，两个入口各一行调用。
> **两个入口的中间件链必须保持一致** —— 拆进程后同一个请求经 nginx 落到哪个进程是不定的，
> 两边链路不同会让同一次调用表现出不同的清洗/脱敏行为，而那种差异极难从现象反推。
> 收进 `Register` 就是为了让这件事 **物理上无法**做错：加 `Auth` 时只改一个文件，两个进程自动同步。

#### 加解密的顺序依据

`ApiEncrypt` 之所以必须排在 `RepeatableBody` 前面，依据是 Java 侧的 Filter `order`（数值越小越先执行）：

| Filter                         | order                                | 实际次序        |
|--------------------------------|--------------------------------------|-----------------|
| `CryptoFilter`                 | `HIGHEST_PRECEDENCE`                 | 最先            |
| `XssFilter`                    | `HIGHEST_PRECEDENCE + 1`             | 其次            |
| `RepeatableFilter`             | 未指定 = `LOWEST_PRECEDENCE`         | 最后            |
| `PlusWebInvokeTimeInterceptor` | Interceptor，在 DispatcherServlet 内 | 晚于所有 Filter |

关键在 `DecryptRequestBodyWrapper.java:93` —— 它的 `getContentType()` **恒返回 `application/json`**， 于是
`RepeatableFilter` 的 `startsWith(application/json)` 判定通过、会在解密包装外面再包一层：

```
RepeatedlyRequestWrapper( DecryptRequestBodyWrapper( 原始 request ) )
```

也就是说 **Java 侧拦截器读到的是解密后的明文**，那边的脱敏是真的作用在明文上。顺序搞反的后果：

- 放在 `AccessLog` 之后 → 日志里永远只有密文，脱敏形同虚设，且 handler 绑不到参数（body 已被吃掉）。
- 放在 `RepeatableBody` 之前（本实现）→ 日志是明文、脱敏正常生效，但这也意味着 **明文密码会流进 `jsonParamLog`**，
  `removeSensitiveFields` 那条路径必须靠得住 —— 这正是 `logger.go` 的 `rawParamLog` 有意偏离 Java、对非法 JSON
  也要做敏感字段探测的原因（四个强制加密接口全都在传密码）。 由
  `TestAPIEncryptDecryptedBodyGetsSanitizedInLog` 锁住。

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
  这是原项目唯一的请求关联手段（因为没有 traceId）。Go 侧 **两者并存**：errorId 仍进响应体，traceId 进日志前缀，
  指向同一条日志；不替换是为了不改返回文案（前端已能从 `X-Request-Id` 响应头拿到 traceId）。
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

#### Go 实现的有意偏差（`cors.go`）

| 位置             | 偏差                                   | 原因                                                                                            |
|------------------|----------------------------------------|-------------------------------------------------------------------------------------------------|
| 校验失败         | 回真实 **403**，不是恒 200             | 跨域校验失败在浏览器 CORS 协议层，响应体被浏览器吞掉，前端读不到 `body.code`，回 200 反而误导   |
| 配置来源         | 走 `pkg/config` 的 `middleware.cors.*` | 原项目 yaml 里没有 `web.cors`，实际生效的是代码默认值；Go 侧把这些值提到了 yaml，默认值与之一致 |
| `ExposedHeaders` | 新增字段，默认含 `X-Request-Id`        | Java 侧没设这项。不加前端拿不到 traceId，无法和服务端日志对账 —— 与 `trace.go` 配套             |
| 通配匹配         | 自己扫 `*` 分段，未用正则              | pattern 来自配置文件非用户输入，逐段扫描够用，且省掉 `*`→`.*` 转义的边界问题                    |

一条容易写错的地方：`allowedOriginPatterns` 命中后要回显 **请求带来的 Origin**，不是配置里的 pattern。 同理
`allowedMethods` 配 `*` 时回显请求的方法（对齐 `checkHttpMethod` 在 ALL 时返回 `singletonList(requestMethod)`），
`allowedHeaders` 配 `*` 时逐个回显请求头。

`Vary: Origin / Access-Control-Request-Method / Access-Control-Request-Headers` 三个头 **无条件加**（同源请求也加）， 对齐
`DefaultCorsProcessor.processRequest` —— 它在判断是否跨域 **之前**就加。不加会让 CDN/代理把 A 站点的跨域头缓存给 B 站点。

预检判定必须是 `OPTIONS` **且**带 `Access-Control-Request-Method` 两个条件（对齐 `CorsUtils.isPreFlightRequest`）， 只看
method 会把普通 OPTIONS 探测误判成预检。预检命中后 `AbortWithStatus(200)` 就地结束，不透给业务路由 —— 对齐 `CorsFilter` 里
`isPreFlightRequest` 就 return、不调 `filterChain.doFilter`。

### 3. TraceID：原项目没有，别去找

全项目零 `MDC.put`、零 `TransmittableThreadLocal`、零 Micrometer Tracing / Sleuth。
`ruoyi-admin/src/main/resources/logback-plus.xml:104,114` 有 `%tid` 但 **被注释掉了**（残留的 SkyWalking/TLog 配置）。 生效的
pattern 只有 `%d{...} [%thread] %-5level %logger{36} - %msg%n`。

所以这块 **没有对照物**，是 Go 侧自主设计。落地方案（`trace.go`）：

- 头名取 `X-Request-Id` —— nginx `$request_id`、各家网关和前端 axios 拦截器的既有约定，接入成本最低。
- id 为 **32 位十六进制**（16 字节），对齐 W3C Trace Context 的 trace-id 与 nginx `$request_id`，将来接 OpenTelemetry /
  SkyWalking 不用换格式。用 `math/rand/v2` 而非 `crypto/rand`：链路 id 只要「不重复」，不要「不可预测」（它不是凭证）。
- 同时存 `gin.Context`（键 `traceId`，给 handler）和 `request.Context()`（私有 key，给 service/repository 层用
  `TraceIDFrom(ctx)` 取， **不必 import gin**）。
- **响应头必须在 `c.Next()` 之前写**：body 一开始输出，header 就已经发出去了，事后 `Set` 会无声失效。

#### 两个容易漏掉的配套点

1. **入站 id 是不可信输入**，必须过 `sanitizeTraceID` 白名单（`[0-9A-Za-z_-]`，≤64 字符）后才能沿用，不合规就丢弃重新生成。 带
   CR/LF 的 id 原样写进响应头就是 **头注入**，写进日志就是 **伪造日志行**；超长的能零成本撑爆日志。
   采取白名单而非过滤坏字符 —— 过滤会把两个不同的入站 id 折叠成同一个，反而制造出对不上的链路。 需要完全不信上游时（进程直接暴露公网）把
   `TrustInbound` 设 false。
2. **`X-Request-Id` 必须在 CORS 的 `ExposedHeaders` 里**，否则跨域下浏览器挡住这个头，前端拿不到 traceId 就无法和服务端日志对账。
   `DefaultCORSConfig()` 已加，这是相对 Java 侧（该项为空）的 **有意新增**，改一处要想到另一处。

`Recover` 的日志已带 `[traceId]` 前缀（`logTracePrefix`）。8 位错误编号 **保留不动**：编号进响应体、traceId
进日志前缀，两者指向同一条日志。没把 traceId 拼进 message 是因为那属于行为变更，而前端本来就能从响应头拿到它。

### 4. 可重复读 Body：只包 JSON，且必须自己设上限

`RepeatableFilter.doFilter` 的判断只有一条：`contentType` 以 `application/json` 开头就换成
`RepeatedlyRequestWrapper`，后者在构造时 `IoUtil.readBytes` 一次读完存 `byte[]`，`getInputStream()`
每次基于它新建 `ByteArrayInputStream`。Go 里等价实现是「读出来，再塞个新 Reader 回去」。

三个容易搞错的边界：

- **不按请求方法过滤**。跳过 GET/DELETE 是 `XssFilter` 的行为，两个 filter 别搞混 —— 带 JSON body 的 `DELETE`
  是合法的，按方法排除会让它读不到参数。
- **不要把 `multipart/form-data` 加进 `ContentTypes`**。那会把上传文件整个读进内存，原项目允许 10MB 单文件 / 20MB
  单请求，并发几个就够压垮进程。
- **表单请求不需要缓存**。`net/http` 的 `ParseForm` 会把结果缓存进 `r.PostForm`，后续读的是解析结果而非 body，天然可重复 ——
  这与 Java 侧 `AccessLog` 走 `getParameterMap()` 而非读 body 是同一个道理。

#### Go 实现的有意偏差（`body.go`）

| 位置               | 偏差                              | 原因                                                                                                                                                            |
|--------------------|-----------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 大小上限           | **新增** `MaxBodySize`，默认 10MB | Java 侧无上限（`max-http-form-post-size` 只管表单不管 JSON），靠前置 nginx `client_max_body_size` 兜；Go 侧无上限的 `io.ReadAll` 等于让调用方决定进程吃多少内存 |
| 超限处理           | 拒绝整个请求，不截断              | 截断会让 handler 拿到半截 JSON，报出一个跟真实原因毫无关系的解析错误                                                                                            |
| `gin.BodyBytesKey` | 额外写一份                        | 让 handler 能用 `c.ShouldBindBodyWith` 复用缓存、多次绑不同结构体；与 `BodyBytes` 共用底层数组，无额外拷贝                                                      |
| 空 body / nil      | 不设缓存键                        | 缺键与空 body 对 gin 等价（都让 `ShouldBindBodyWith` 报 EOF），少存个键省得让人误以为缓存过                                                                     |

读 body 的中间件一律走 `BodyBytes(c)`， **取不到就跳过，不要退回去读 `c.Request.Body`** —— 那会把 body 吃掉，handler
再绑参数就是空的。这正是 Java 侧 `PlusWebInvokeTimeInterceptor` 里 `if (request instanceof RepeatedlyRequestWrapper)`
的用意：宁可少打日志，也不能把 body 吃掉。返回的是缓存本身而非副本，调用方 **不要修改**。

### 5. 请求日志：脱敏和截断都要照做

`PlusWebInvokeTimeInterceptor` 的三个细节：

- `preHandle` 打 `[PLUS]开始请求 => URL[...],参数...`，`afterCompletion` 打 `[PLUS]结束请求 => URL[...],耗时:[N]毫秒`， 用
  `ThreadLocal<StopWatch>` 计时（Go 直接在中间件闭包里存 `time.Now()`，比 ThreadLocal 简单）。
- **脱敏**：JSON body 和 query 参数都会剔除 `SystemConstants.EXCLUDE_PROPERTIES`。 本项目已移植为
  `constant.ExcludeProperties`（`pkg/constant/system.go:21`），直接用，别另写一份。
- **截断**：参数日志最长 4000 字符。

分两行打是刻意的，别为了省日志量合成一行：只打结束行的话，请求卡死或把进程搞挂时日志里什么都不会留下。 结束行走 `defer`
而非 `c.Next()` 之后直接打 —— 后续中间件或 handler panic 时栈会一路展开到最外层的 `Recover`，中途不经过这里， 只有 `defer`
能保证「开始」必有「结束」（对齐 `afterCompletion` 在 `ex != null` 时同样被调用）。

#### Go 实现的有意偏差（`logger.go`）

| 位置         | 偏差                                          | 原因                                                                                                                                 |
|--------------|-----------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| 非法 JSON    | 含敏感字段名则整段丢弃，不回原文              | Java 的 `sanitizeJsonParam` 解析失败直接返回原文，一个少引号的 body 就能把明文密码带进日志 —— 日志留存远长于会话，是实打实的泄漏口子 |
| 数字解析     | `Decoder.UseNumber()`                         | 默认解成 `float64` 只有 53 位有效位，19 位雪花 id 会被抹掉尾数，打出来的假 id 查库查不到，比不打更误导人                             |
| 截断         | 按 rune 截并加 `...(已截断)` 标记             | 按字节截会把中文劈成乱码；不留标记的话，被砍断的 JSON 看起来就是非法 JSON，排查的人会去追一个不存在的解析问题                        |
| 脱敏体积上限 | **新增** `maxSanitizeSize`(256KB)，超限走原文 | 解析结果只为打日志，而日志最多留 4000 字符，为 10MB body 建满树再序列化，成果绝大部分立刻丢掉                                        |
| `SkipPaths`  | **新增**，默认为空                            | Java 注册在 `/**` 无开关。探针每几秒一次、零信息量，不排除会把有用日志冲走。默认空即与原项目一致                                     |
| 结束行状态码 | **新增** `状态[%d]`                           | 业务码恒回 200 时这字段没信息量，但正好暴露那几条不走业务码的路径：CORS 拒绝的 403、未命中路由的 404/405                             |
| 日志前缀     | 带 traceId                                    | Java 侧「开始」「结束」两行在并发下根本没法配对（全项目无 traceId）                                                                  |
| multipart    | 不解析表单，只打查询串                        | 解析要把上传文件整个读进内存，代价远超一行日志的价值；相比 Java 的 `getParameterMap()` 会少掉表单字段                                |

两个和 `body.go` 配套、改一处要想到另一处的点：

1. **JSON 入参只从 `BodyBytes(c)` 取，取不到就打空参数**，绝不回头读 `c.Request.Body`。`isJSONRequest` 的判定与 `body.go`
   的缓存判定同源，那边不缓存的这边也取不到 —— 这正是 Java 侧 `if (request instanceof RepeatedlyRequestWrapper)` 的用意。
2. **query 脱敏必须复制一份 `Request.Form` 再删**，直接 `delete` 会让业务真的收不到这个参数。 同理日志里打的是
   `URL.Path` 而非 `RequestURI` —— 后者带原始查询串，直接输出等于把 `?password=xxx` 原样写进日志，脱敏全白做。

### 6. XSS：开关 + 两级跳过，以及原项目一个会破坏 JSON 的 bug

注册在 `ruoyi-common-web/…/web/config/FilterConfig.java:29-39`，`@ConditionalOnProperty` 挂 `xss.enabled`。

- **跳过 GET / DELETE**（filter 内部判 method 直接放行）
- **跳过 `xss.excludeUrls`**：现为 `/system/notice`、`/warm-flow/save-json`（富文本/JSON 存原文，过滤会破坏内容）
- 配置类 `web/config/properties/XssProperties.java`，yaml 在 `application.yml:190-196`

清洗逻辑本身是 hutool 的 `HtmlUtil.cleanHtmlTag`，即拿 `RE_HTML_MARK` 把标签替换成空串（ **保留**标签内的文字）。 该正则原值是三段或
`(<[^<]*?>)|(<[\s]*?/[^<]*?>)|(<[^<]*?/[\s]*?>)`，但后两段 **完全被第一段吞掉**（第一段的 `[^<]*?` 已能匹配
`/p`、`br/`、` /p`）。Go 侧只留 `<[^<]*?>`，并由 `TestCleanHTMLTagMatchesHutoolRegex` 用固定用例 + 3000 条随机串与原正则交叉验证，
确认行为无差异。惰性量词 `*?` 不能改成贪心 —— 那会把 `<b>x</b>` 连中间的 `x` 一起吃掉。

#### 首先要说清楚它不是什么

剔标签是 **纵深防御的一层，不是主要防线**。它拦不住 `javascript:` 协议、事件属性拼接、HTML 实体编码的载荷， 也管不了从 DB
读出来再渲染的老数据。真正的防线在输出侧（前端渲染转义 / 后端返回 JSON 而非 HTML 片段）。

**GET / DELETE 整体跳过**这条本身就是个真实缺口：带 XSS 载荷的 GET 查询参数不经任何清洗。原项目如此，本包对齐 （
`TestXSSSkipsGetAndDelete` 把它锁成显式的既有行为，而不是让它看起来像漏了）。
真靠这个中间件兜底的话，这个缺口早该是个事故了 —— 它的存在恰好证明了防线在别处。

#### Go 实现的有意偏差（`xss.go`）

| 位置            | 偏差                                        | 原因                                                                                                                                                 |
|-----------------|---------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| **JSON 体清洗** | **逐值清洗**，不对整串做正则替换            | Java 对整个 JSON 串调 `cleanHtmlTag`，**会破坏 JSON 结构**，详见下方。这是 bug，不是可对齐的行为                                                     |
| JSON 键         | 不清洗，只清洗字符串值                      | 键名带标签的合法请求不存在；清洗键会让两个不同键撞成同一个，静默丢字段                                                                               |
| JSON 值         | 不做 `trim`                                 | Java 只对整串 trim 了一次（只动首尾空白，对 JSON 语义无影响），并未逐值 trim                                                                         |
| 非法 JSON       | 原样放行，不退回字符串替换                  | 这种 body 必然过不了 `ShouldBindJSON`，请求会被拒、载荷到不了落库；而对非法 JSON 做正则替换只会制造结构损坏                                          |
| 体积上限        | **有意不设**                                | `logger.go` 的 `maxSanitizeSize` 超限走原文，代价只是日志难看；这里跳过清洗 = 跳过一层防御，攻击者填满阈值即绕过。总量由 `RepeatableBody` 的 10MB 兜 |
| 数字            | `Decoder.UseNumber()`                       | 与 `logger.go` 同因但更严重：那边打错日志是误导人，这边是**改请求** —— 19 位雪花 id 被 float64 抹掉尾数就是把主键改成不存在的值                      |
| 查询串          | 改写 `URL.RawQuery`                         | Java 覆写 `getParameter` 一次拦下查询串与表单；Go 里两者分处 `URL.RawQuery` / `PostForm`，且 `c.Query` 只认现场解析的 `URL.Query()`                  |
| 查询串解析失败  | 保持原样，不用部分结果重新编码              | `url.ParseQuery` 出错时返回的是**部分**结果，拿它重编码会静默丢掉解析失败的参数，比不清洗更难排查                                                    |
| `multipart`     | 不处理表单文本字段                          | `ParseMultipartForm` 要把上传文件整个读进内存（单文件 10MB），代价远超清洗几个字段；与 `body.go` 同一取舍。相比 Java 会少清洗这些字段                |
| 日志顺序        | XSS 在 `AccessLog` **之后**，日志记原始报文 | Java 的 `XssFilter`（order `HIGHEST+1`）跑在 `RepeatableFilter` 前，拦截器读到的是清洗后的 body。日志是排查与取证用的，需要看到攻击者到底发了什么    |
| `xss.enabled`   | 无此配置项                                  | Go 侧「注册」就是 `middleware.Register` 里那行 `r.Use(XSS())`，不写即关闭。再加布尔开关只会造出「注册了但不生效」这种要翻两处才能确诊的状态          |

`middleware.xss.*` 现在 **已经在 yaml 里**（`excludeUrls` / `skipMethods` 可配），但 **仍然没有 `enabled`** —— 这不是漏了。
上表那条理由不因「配置进了 yaml」而失效：开关与注册是两套机关，同时存在就会出现「yaml 里 `enabled: true` 但
`Register` 没挂它」这种要翻两处才能确诊的状态。要关掉某一环，改 `register.go` 删那一行。 这条对本节这 6 个中间件一律适用，不是
XSS
的特例。

唯一的例外是 `apiEncrypt`（第 8 节），它 **有** `enabled` —— 因为它不生效时请求会被当明文交给 handler、
报出一个与真实原因无关的参数错误，必须能区分「没开」与「开了但密钥错」。理由详见那一节。

#### 那个会破坏 JSON 的 bug

`XssHttpServletRequestWrapper.getInputStream()` 是 `HtmlUtil.cleanHtmlTag(整个 JSON 字符串)`。实测：

```
{"a":"1<2","b":"3>4"}   ->   {"a":"14"}
```

正则把 `<2","b":"3>` 整段当成一个标签吃掉了，`b` 字段 **凭空消失**。触发条件很宽：只要某个字符串值里有 `<`、 后面任意位置还有
`>`，两者之间的所有内容（字段名、逗号、引号）就会被抹掉，handler 绑到的是一个结构被改过的对象。
数值比较、不等式这类正常业务文本 （`"库存 < 10"`、`"a>b"`）就足够踩中。

Go 侧改为「解析成树 → 只清洗字符串值 → 序列化回去」，结构不可能被破坏。 `TestXSSDoesNotCorruptJSONStructure` 同时断言 Go
侧保留两个字段、 **且** hutool 正则确实会产出上述损坏形态 —— 后半条是为了在假设变化时（比如将来换清洗实现）让用例前提失效得明显。

无改动时跳过重新序列化：大多数含 `<` 的请求（`1 < 2`）并没有真标签，重新编码只会打乱字段顺序、改写数字格式。

#### 三处必须同时更新

清洗完 JSON 后要一起改 `Request.Body`、`gin.BodyBytesKey`、`Request.ContentLength`： 分别对应 `ShouldBindJSON`、
`ShouldBindBodyWith`、以及按长度读 body 的一方。 漏掉任一处就会让某条读取路径拿到未清洗的数据或读到错误长度 （
`TestXSSUpdatesContentLengthAndBodyCache` 覆盖）。 表单同理要同时清洗 `Request.Form` 与 `Request.PostForm`
两个 map —— gin 的 `c.PostForm` 与 `binding.Form` 分别读其中之一。

与 `logger.go` 正好相反的一点：那边脱敏 **必须复制一份再删**（日志绝不能动请求），这边 **就地改**（目的就是让 handler
收到清洗后的值）。

#### Ant 路径匹配抽成了 `path.go`

`excludeUrls` 用的是 Spring `AntPathMatcher` 语义（`?` 单字符 / `*` 单层 / `**` 任意层），由 `MatchAnyPath` 实现。 **单独放
`path.go` 而非塞进 `xss.go`**：阶段 1 的 `security.excludes`（`application.yml:100-113`，含 `/**/*.html` 这类跨层
pattern）是同一套语义，`auth.go` 会直接复用。 两处各写一份的话，免过滤名单与免鉴权名单迟早在边界行为上分叉 ——
那是安全配置，不能靠巧合对齐。

两个实现要点：空规则集必须返回 `false`（「没配排除规则」不能变成「全部排除」，取反就是敞开的口子）；
`**` 的匹配用 DP 备忘录而非朴素递归 —— 后者在 pattern 含多个 `**` 时是指数级的，而 path 段数由请求方决定， 那就成了一条用
URL 长度换 CPU 的放大路径（`TestAntPathMatchNoExponentialBlowup` 兜住）。 匹配用 `URL.Path` 而非 `RequestURI`，对齐 Java 的
`getServletPath()`：后者带查询串，会让
`/system/notice?x=1` 匹配不上 `/system/notice` 这条排除规则。

### 7. i18n：从 `content-language` 取，不是 `Accept-Language`

`web/core/I18nLocaleResolver.java` 读的是 **`content-language` 请求头**（非标准用法，但要对齐前端）， 下划线归一成横线，取不到回落
`Locale.getDefault()`。`setLocale` 是刻意的空实现。 词条目录由 `spring.messages.basename: i18n/messages` 指定。

按 RFC 9110，`content-language` 描述的是 **报文自身内容** 的语言（「我这个请求体是中文写的」），表达「请把响应翻成中文」的标准头是
`Accept-Language`。原项目用错了头，但前端发的就是它，头名对不上等于整个 i18n 不生效，故对齐。

**有意不兼容 `Accept-Language` 回落**：浏览器会 **自动** 带上它（通常是操作系统语言）。一旦回落过去，用户在前端切成英文后，只要某个请求漏发
`content-language`，就会拿到跟界面语言不一致的文案 —— 这种「偶尔一句中文」极难定位。宁可只认一个显式来源。
`TestI18nReadsContentLanguageNotAcceptLanguage` 把这条锁住。

#### 词条落在 `pkg/i18n` 而非本包

中间件只负责 **解析语言并写进 context**（十几行）；词条表与渲染在 `pkg/i18n`，对应 Java 的 `MessageUtils` +
`messages*.properties`。 分开是因为 service / repository 层要取文案，但那些层不该 import gin。

Java 的当前语言由 `LocaleContextHolder` 这个 ThreadLocal 隐式提供，所以 `MessageUtils.message(code, args)` 不必传语言。 Go
侧 **显式随 `context.Context` 传**：`i18n.Msg(ctx, code, args...)`，与 `trace.go` 的 `TraceIDFrom` 同一套做法 ——
少一个隐式全局状态，goroutine 里也不会莫名丢语言。

**54×2 条文案是手工搬过来的**，抄错一个字、漏一条、把 zh 的值贴到 en 里，都不会有编译错误，表现出来只是「某个提示的文案不对」——
那是没人会去核对的东西。故 `pkg/i18n/i18n_test.go` 的 `TestCatalogsMatchJavaProperties` **直接读原项目的 `.properties`
逐条交叉验证**（占位符折算回去后要求完全一致）。原项目路径不存在时该用例 Skip 而非 Fail，好让没有原项目的机器也能跑通构建。

#### Go 实现的有意偏差

| 位置                   | 偏差                                            | 原因                                                                                                                                                                  |
|------------------------|-------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 词条存储               | 嵌进 Go 源码的 `map`，不读 `.properties`        | 每模块编译成独立 binary，词条编进去则拷一个文件就能跑；漏拷 `resources` 目录这种故障要等某条错误路径被触发才暴露，而那通常是线上                                      |
| 占位符                 | `{0}`/`{1}` → `%v`，渲染走 `fmt.Sprintf`        | 与 `errs.Newf`、`enum.LoginType` 已有的模板一致；再引入一套 `{}` 解析会让同一仓库里有两种占位符风格                                                                   |
| `{min}`/`{max}`        | **保持原样不转换**                              | 那不是 MessageFormat 位置参数，是 Hibernate Validator 的**属性占位符**，由注解的 min/max 回填，Java 侧也不经 MessageFormat                                            |
| 默认语言               | 固定 `zh-CN`，不读操作系统区域                  | Java 回落的 `Locale.getDefault()` 跟着 **JVM 所在机器** 走；多副本部署下节点由 nginx 随机挑，同一请求会因落到哪台而返回不同语言                                       |
| `content-language: en` | 回落 **英文**（`en-US`）                        | 原项目缺 `messages_en.properties`，ResourceBundle 的查找链会退到系统默认区域，中文机器上 `en` 实际**返回中文** —— 那是文件缺失导致的意外，不是设计意图                |
| 非法语言标记           | 走白名单校验后回落默认，**不拒绝请求**          | Java 的 `forLanguageTag` 对非法输入静默返回 `und`。Go 加校验是因为这个值会进日志，带 CR/LF 能伪造日志行；但语言只影响文案呈现，为一个畸形的头打回整个业务请求不成比例 |
| 列表形态               | 按逗号 **取第一段**（`en-US, zh-CN` → `en-us`） | `content-language` 按 RFC 9110 可以是列表，Java 的 `forLanguageTag` 遇到它解析成 `und` 从而回落默认语言，这里比原项目多支持一步                                       |
| 裁空白                 | 只裁空格与 TAB，**不用 `TrimSpace`**            | `TrimSpace` 会裁掉 `\r\n`，于是 `"zh-CN\r"` 被悄悄修好成合法值，而白名单本来就是要挡住它的 —— 带 CR 的头意味着上游有问题，值得暴露                                    |
| 响应头                 | **新增** 回显 `Content-Language`                | 原项目不回。归一化（`zh-Hans-CN` → `zh-cn`）只看响应体看不出来；出现「明明发了 en 却收到中文」时，这个头能一眼区分是语言协商的结果还是词条缺失                        |
| `setLocale`            | 不提供                                          | Java 的 `LocaleResolver` 接口强制实现，原项目给了空实现（服务端不主动切语言）。Go 没有这个接口约束，少一个「存在但什么都不做」的方法                                  |
| 注册词条的入口         | **有意不提供**                                  | 那需要一个可变全局 `map`，而中间件每请求都读它 —— 启动后再写就是数据竞争。新增语言直接加一个 `messages_xx.go` 文件                                                    |
| 词条缺失               | 返回 `code` 本身                                | 对齐 Java `catch NoSuchMessageException` 后 `return code`。返回空串会让前端显示一片空白，无从判断是「没有提示」还是「词条漏了」                                       |
| 无参调用               | 返回原始模板，不过 `Sprintf`                    | 过 `Sprintf` 会渲染成 `%!v(MISSING)`。「该带参数却没带」是调用方的 bug，不该在这里加工成一句更难看的文案；对齐 MessageFormat 无参时保留 `{0}`                         |

#### 顺序：只有一条约束

**必须在 `Auth` 之前** —— 阶段 1 鉴权会返回「客户端ID与Token不匹配」这类文案，那些要走词条，就得先有语言。 它不读
body、不改请求，与前面几环没有耦合。

位置靠后不影响前面的中间件：`Recover` 与 `AccessLog` 的输出是 **日志**，面向运维、恒为中文，本来就不该跟着请求语言变 ——
否则同一个错误在日志里有两种文案，检索时得搜两遍。

和 `trace.go` 共有的两条纪律： **响应头必须在 `c.Next()` 之前写**（body 一开始输出 header 就发出去了，事后 `Set` 静默失效，由
`TestI18nHeaderSetBeforeHandlerWritesBody` 兜住）； **用 `WithContext` 替换 `c.Request` 时必须基于前一环的 context**， 否则会把
`TraceID` 写进去的值覆盖掉（`TestI18nCoexistsWithTraceID` 覆盖）。

### 8. 接口加解密：唯一带 `enabled` 开关的中间件

注册在 `ruoyi-common-encrypt/…/encrypt/config/ApiDecryptAutoConfiguration.java`，`@ConditionalOnProperty` 挂
`api-decrypt.enabled`，yaml 在 `application.yml:148-158` —— **该值是 `true`**，前端现在就在加密这几个接口，不是假设问题。

#### 协议

请求方向（前端 → 服务端），注意 AES 密钥被 base64 **套了两层**：

```
encrypt-key 头 = base64(RSA公钥加密( base64( AES明文密钥 ) ))
请求体         = base64(AES-ECB加密( JSON 明文 ))
```

里层那层 base64 是 `EncryptUtils.encryptByBase64`/`decryptByBase64`，看着多余但它是协议的一部分（前端照此实现），少一层就对不上。
响应方向与之对称，AES 密钥换成服务端每次请求新生成的一次性密钥、用 **前端的公钥**加密后放 **同名头**（请求与响应共用
`encrypt-key` 这一个头名，只是载荷方向不同）。

#### 命中方式：注解 → 配置清单

Java 的 `CryptoFilter` 注册成全局 filter，但内部通过 `RequestMappingHandlerMapping` 反查 `@ApiEncrypt` 注解，只对标注的方法生效。
Go 没有注解，改成配置里的 Ant 路径清单。原项目 **4 处** `@ApiEncrypt` 全部对应到 `apiEncrypt.requestUrls`：

| 接口                             | Java 位置                      | 方法 |
|----------------------------------|--------------------------------|------|
| `/auth/login`                    | `AuthController.java:75`       | POST |
| `/auth/register`                 | `AuthController.java:180`      | POST |
| `/system/user/resetPwd`          | `SysUserController.java:249`   | PUT  |
| `/system/user/profile/updatePwd` | `SysProfileController.java:86` | PUT  |

四个全在传密码，所以 **`PUT` 必须和 `POST` 一样解密**（对齐 `CryptoFilter` 的 `PUT || POST` 判断）—— 漏掉 PUT
会让两个改密码接口完全不可用。

清单的语义是 **强制**：命中却没带 `encrypt-key` 头就拒绝（对齐 Java 里「有注解却无加密标头就报 403」的分支），
拦的是「本该加密的接口收到了明文密码」。反之 **带了头就解密，与清单无关** —— 对齐 Java，那边解密只看头、不看注解。

> `@ApiEncrypt` 的 `response()` 默认 `false`，而原项目 4 处 **全部用的默认值** ——
> 也就是说 **响应加密在原项目里从未被启用过**。本包照 `EncryptResponseBodyWrapper` 复刻了它，
> 但 `responseUrls` 默认为空、且没有可对照验证的线上行为。开启前先确认前端确实实现了响应解密。

#### 为什么这一个有 `enabled`

其余 6 个中间件 **有意没有** `enabled`（「注册即启用」，见上方 XSS 一节）。这一项是例外，因为它的失败方向相反：

- XSS / AccessLog 不注册 = 少一层清洗或少几行日志，请求照常处理。
- 本中间件不生效 = 带 `encrypt-key` 头的请求被 **当作明文**交给 handler，JSON 解析必然失败， 前端收到一句莫名的参数错误 ——
  而真正的原因（服务端没开解密）在报文里没有任何痕迹。

即必须能区分「没开」与「开了但密钥配错」，`enabled` 就是那个区分。这也是 `configs/application.yaml`
里 **唯一与代码默认值不同**的一节（yaml 是 `true`，代码默认 `false`）：默认值要在「没有配置文件」时也讲得通， 而启用状态下缺私钥必须报错，默认
`true` + 默认空私钥会让任何未配置的进程启动失败。 这个差异由
`pkg/config` 的 `TestRealYAMLEnablesAPIEncrypt` 显式锁住，`TestRealYAMLMatchesDefaults` 把这一段排除在外。

#### 失败一律折叠成同一句文案

**所有解密失败回同一句「请求解密失败」**，不区分是 RSA 阶段、base64 阶段、AES 密钥长度还是 PKCS#7 填充校验失败。 这不是偷懒 ——
区分失败原因就等于提供一个 **padding oracle**：攻击者能拿同一段密文反复试探， 靠「填充错」与「解密错」两种不同回复逐字节还原明文（Vaudenay
攻击）。真实原因只进日志。 由 `TestAPIEncryptFailuresAreIndistinguishable` 锁住（9 种畸形输入断言回同一句）。

#### Go 实现的有意偏差（`crypto.go`）

| 位置         | 偏差                              | 原因                                                                                              |
|--------------|-----------------------------------|---------------------------------------------------------------------------------------------------|
| 命中方式     | 配置路径清单，非注解反查          | Go 无注解；Java 靠 `RequestMappingHandlerMapping` 查 `@ApiEncrypt`                                |
| 失败文案     | 全部折叠成一句                    | 区分失败原因 = 提供 padding oracle（见上）                                                        |
| 失败状态码   | 恒 200 + 业务码                   | 走 `c.Error` 由 `Recover` 统一渲染，与其余接口一致；Java 那边 `resolveException` 回真 403         |
| 密钥解析     | **启动期一次**，捕获进闭包        | Java 每请求重新 `KeyFactory.generatePublic` 解析 ASN.1；顺带把「密钥格式错」从运行期挪到启动期    |
| 体积上限     | **新增** `maxBodySize`，默认 10MB | Java 侧无上限（`IoUtil.readBytes` 读到底），与 `body.go` 同一个放大问题                           |
| 密文编码     | 只认 base64，**不猜 Hex**         | hutool 的 `SecureUtil.decode` 用 `isHexNumber` 猜编码，一段恰好只含 `[0-9a-f]` 的 base64 会被误判 |
| 空响应       | 不加密，原样放行                  | 加密空串会产出一个「解开是空」的密文，比空响应更让人困惑                                          |
| 错误响应     | **不加密**，且撤掉密钥头          | 见下方「一处顺序上无法两全的地方」                                                                |
| 响应加密失败 | 丢弃响应体，回错误 JSON           | 绝不能退回去写明文 —— 那正是要防的                                                                |
| `Flush`      | **有意不覆写**                    | 流式接口（SSE/下载）与整体加密语义不兼容，不该配进 `responseUrls`；加保护反而会掩盖这个配置错误   |

#### 一处顺序上无法两全的地方

handler 走 `c.Error(err)` 而不自己写响应体时（本项目 handler 的标准错误路径），那份响应由最外层的
`Recover` 渲染 —— 而那发生在 `ApiEncrypt` **返回之后**，所以 **无法加密**。此时本中间件会撤掉密钥头，
让前端知道这是明文（留着头会让它拿去解一段明文 JSON，得到一句「解密失败」而非服务端真正想传达的错误）。

Java 侧没有这个问题：`CryptoFilter` 在 filter 链内部，异常经 `handlerExceptionResolver` 渲染后仍会被 wrapper 缓冲并加密。Go
侧 `Recover` 必须在最外层（要兜住所有中间件自身的 panic），两者顺序上无法兼得。 取舍是
**「错误能送达但不加密」而非「加密但送不到」**。

> 这里曾经有一个真实的 bug：替换了 `c.Writer` 却没有还原，于是 `Recover` 那份错误响应写进了已经用完的
> 缓冲区，客户端收到 **200 空响应**、服务端日志里只有一行业务异常 —— 两边都看不出发生了什么。
> 现由 `TestAPIEncryptDeliversHandlerErrors` 锁住。包装 `c.Writer` 的中间件都要注意这一点：
> **用完必须还原**，因为链外还有中间件会往里写。

#### ECB 是照抄，不是选择

`SecureUtil.aes(byte[])` 走 JCE 默认变换 **AES/ECB/PKCS5Padding**（无 IV）。ECB 无 IV、相同明文块恒产出相同密文块，
密文因此暴露明文的重复结构，且可被逐块重排而不被察觉（无完整性保护）。

Go 的 `crypto/cipher` **有意不提供 ECB**，所以 `pkg/encrypt` 里手写了它，并在注释里写明了理由。 不改成 CBC/GCM 是因为这是
**通信协议**而非内部实现：前端的加密实现与这里必须逐字节对齐， 单方面换模式等于把接口打死。要收紧就得前后端一起换。

`TestAESIsDeterministicECB` 把这个性质 **显式记录**下来：将来谁把模式换了，那条用例会失败 —— 那正是提醒「这是协议变更」的地方。

> 前面「未核实项」里那个问题（ECB 的确定性会不会让密文成为可比对的指纹，取决于前端是否每次换 AES key）
> **仍未核实** —— 仓库内无前端代码。但响应侧已确认是每次请求新生成
> （`EncryptResponseBodyWrapper.generateAesPassword`），请求侧按对称推测亦然。
> 本包的响应加密照此实现，并由 `TestAPIEncryptResponseKeyIsPerRequest` 锁住。

#### 跨语言验证到哪一步了

`pkg/encrypt` 的 `TestAESMatchesFIPSVector` 拿 **FIPS-197 附录 C.1** 的官方向量比对，锁住了「分组密码 + ECB 模式」这一层。
往返测试（自己加密自己解密）做不到这件事 —— 模式选错、字节序搞反都能自洽地往返成功，却与前端完全对不上。

**仍未跨语言核实的是 PKCS#7 填充与 base64 编码那两层**：它们由 `EncryptUtils` 的代码阅读推得 （`SecureUtil.aes` 走 JCE 默认的
`AES/ECB/PKCS5Padding`，`encryptBase64` 走 base64）。 理想的验证是拿一段 hutool 真实产出的密文来解，但那需要跑 Java。
**首次前后端联调时应重点验这两层。**

### 9. 鉴权：两道前置跳过 + 四步校验

`SecurityConfig.java:80-119` 注册 sa-token `SaInterceptor` 到 `/**`，排除 `securityProperties.getExcludes()`。每请求做四件事：

1. `StpUtil.checkLogin()` 校验 token
2. header / query 的 `clientid` 与 token extra 里的 clientid 交叉比对，不符抛 `NotLoginException` code `-100`，
   message「客户端ID与Token不匹配」
3. `validateClientAccessRules()`（`:147-166`）按客户端做 **URL 路径白名单**，不符抛
   `NotPermissionException("当前客户端未授权访问该接口路径")`
4. 同方法内按客户端做 **IP 白名单**（`NetUtils.isMatchIpRule`），不符抛「当前客户端IP不在白名单内」

Go 侧在这四步之前还有两道跳过，完整流程（`auth.go`）：

```
0a. 命中 middleware.auth.excludes（Ant 风格）   -> 放行
0b. c.FullPath() == ""（未命中任何已注册路由）  -> 放行，交给 NoRoute 落 404
 1. Authorization 头取 token，验签 + 查 exp     -> 失败 401
 2. 查 Redis 会话（登出/空闲超时即无）          -> 失败 401
 3. clientid 交叉比对                           -> 失败 401
 4. 客户端访问路径 -> 客户端 IP 白名单          -> 失败 403
 5. 通过：滑动续期 + 把 LoginUser 写进两处上下文
```

四步的顺序不能调换：先确认「你是谁」再判「能否访问」，否则未登录的请求会先撞上 403 而非 401， 前端的处理分支完全不同（403
提示无权限、401 跳登录页）。

> **那个坑（`0b` 的来历）**：Java 用 `SaRouter.match(allUrlHandler.getUrls())` 匹配。
> `ruoyi-common-security/…/security/handler/AllUrlHandler.java`
> 在启动时遍历 `RequestMappingHandlerMapping` 收集 **所有已注册路由**（`{pathVariable}` 替换成 `*`）。
> 也就是说 **未注册的路径根本不进鉴权，直接落 404 而非 401**。
> Go 侧用 `c.FullPath() == ""` 判定同一件事 —— gin 在路由匹配阶段就填好它，未命中时是空串，
> 比启动时枚举 `engine.Routes()` 再做 Ant 匹配既简单又精确（那还要处理 `:param` / `*wildcard` 的归一化）。
> 代价是暴露了「哪些路径存在」这一位信息，这是 **对齐原项目的有意选择**，
> 由 `TestAuthUnregisteredPathFalls404NotUnauthorized` 锁住。

#### 登录态原语在 `pkg/auth`，不在本包

本包只做 **策略**（谁要鉴权、失败回什么）；「登录态是什么」在 `pkg/auth`：

| 文件            | 内容                                                    | Java 对照物                        |
|-----------------|---------------------------------------------------------|------------------------------------|
| `login_user.go` | `LoginUser`（22 字段）+ `LoginID()` → `"sys_user:<id>"` | `system.api.model.LoginUser`       |
| `claims.go`     | `Claims`：8 个 extra + `sub`/`exp`/`iat`                | `SaLoginParameter` 的 extra        |
| `token.go`      | `Sign` / `Verify` / `TrimTokenPrefix`                   | `StpLogicJwtForSimple`             |
| `session.go`    | `SessionStore`：`Save`/`Load`/`Renew`/`Delete`          | `PlusSaTokenDao` + token-session   |
| `password.go`   | `HashPassword` / `VerifyPassword`（bcrypt cost 10）     | hutool `BCrypt.hashpw` / `checkpw` |

分包理由与 `pkg/encrypt` 同源：`internal/*/service` 要签发与销毁会话，那条路径与 HTTP 无关、不该 import gin。

两条最值得记住的：

- **`Claims` 必须是具体结构体，不能用 `jwt.MapClaims`**。后者底层 `map[string]any`，数字一律解成
  `float64`（53 位有效位），而 `userId`/`deptId` 是 19 位雪花 id —— 尾数会被 **静默** 抹掉。 与 `xss.go`/`logger.go` 用
  `Decoder.UseNumber()` 挡的是同一个坑，但这里坏掉的是 **身份标识**： 拿被改过尾数的 userId 查库，查不到算走运，查到别人是事故。
  `TestClaimsSnowflakeIDSurvivesRoundTrip`
  同时断言 MapClaims 确实会损坏该值，让前提失效得明显。
- **`Verify` 必须传 `jwt.WithValidMethods`**。不加则 alg 由 token 自己声明，改成 `none` 即免签名、 改成 `RS256` 能让服务端拿
  HMAC 密钥当公钥去验一个自己签的 token。两层各有一道锁 （`pkg/auth` 与本包的 `TestAuthRejectsAlgNone`）。

#### 两个超时的分工

`sys_client` 有两列超时，落到 Go 侧是两道独立的闸：

| 列               | 种子值        | 语义             | 载体                              |
|------------------|---------------|------------------|-----------------------------------|
| `timeout`        | 604800（7d）  | **绝对**有效期   | JWT 的 `exp`（签发即冻结）        |
| `active_timeout` | 1800（30min） | **滑动**空闲超时 | Redis 会话的 TTL，每请求 `EXPIRE` |

对应原项目 sa-token 的 `active-timeout` + `dynamic-active-timeout: true`（那边每请求更新
`last-activity` 键，Go 侧直接对会话键续期，少一个键）。
`ActiveTimeout` 存在会话里而非 claims 里：claims 逐字对齐 Java 的 8 个 extra（那是协议）， 而续期时长是服务端的实现细节 ——
放会话侧则改配置对 **新会话**立即生效。

Redis 键布局（ **有意不对齐 sa-token 那 4 个键**，那套是框架内部约定且值是 Jackson 带 `@class` 的多态格式， 复刻它只有「与运行中的
Java 进程共用会话」这一个好处，而本项目是重写不是双跑）：

```
auth:token:<jwt>        -> Session(JSON)   TTL = activeTimeout，每请求滑动续期
online_tokens:<jwt>     -> 在线用户记录     TTL = timeout
pwd_err_cnt:<username>  -> 密码错误计数     TTL = lockTime，每次失败重置（滑动窗口）
```

后两个前缀沿用原项目的名字（`pkg/constant/cache_names.go` 已有），那两个是 **业务**键、原项目里也是自己拼的串， 保持同名以便数据可对照。

#### Go 实现的有意偏差（`auth.go`）

| 位置                | 偏差                                | 原因                                                                                                                                           |
|---------------------|-------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| token 来源          | **只从 header 取**                  | sa-token 配了 `is-read-body: true`（可从请求参数读）。查询串会流进 nginx accesslog 与浏览器历史；cookie 则是原项目刻意关掉的（见下）           |
| 缺 `clientid` claim | 返 **401**                          | Java 的 `StpUtil.getExtra(CLIENT_KEY).toString()`（`SecurityConfig.java:100`）对这种 token **NPE 成 500** —— 那是 bug 不是可对齐的行为         |
| 失败响应            | 走 `c.Error` 由 `Recover` 渲染      | HTTP 状态码恒 200、业务码放响应体，与其余接口一致（`recover.go` 的硬约束）；Java 那边 advice 返回 `R<Void>` 也是 200                           |
| 未注册路径          | `c.FullPath() == ""` 判定           | Java 靠启动时枚举路由表（`AllUrlHandler`），Go 用 gin 现成的字段，不必处理 `:param` 归一化                                                     |
| 失败文案            | 只保留 2 句（Java 有 5 句）         | `BE_REPLACED`/`KICK_OUT` 需要「为什么没了会话」这一层信息，而本实现删会话时不留因由。阶段 3 做在线用户管理时再加，不摆永远走不到的分支         |
| 文案来源            | **常量，不走 i18n 词条**            | Java 侧这几句就硬编码在 `SaTokenExceptionHandler` 里（不走 `MessageUtils`），`pkg/i18n` 也确实没有这些键。凭空新增词条会造出一份无从核对的差异 |
| 客户端规则          | 从 JWT claims 读，**不实时查库**    | 与 Java 一致。每请求查一次 `sys_client` 会把鉴权变成带 DB 依赖的热路径。代价是改了客户端规则要等存量 token 过期                                |
| 续期失败            | 只打日志，不中断请求                | 校验已经通过了，此刻因一次 Redis 抖动把已登录用户挡在门外不成比例；代价只是这次没延长空闲窗口                                                  |
| Redis 故障          | 兜成系统异常（500），**不是 401**   | 回 401 会让一次抖动表现成「所有人被登出」，而日志里只有一片 401 —— 看不出真正的原因。由 `TestAuthRedisFailureIsNotUnauthorized` 锁住           |
| 登录用户存放        | 同时写 `gin.Context` 与 request ctx | 与 `trace.go`/`i18n.go` 同一套做法：后者让 service/repository 层不必 import gin（阶段 4.1 的数据权限要用）                                     |

`is-read-cookie: false` 那条（`ruoyi-common-satoken/src/main/resources/common-satoken.yml`）是 **刻意关掉 cookie 认证来消除
CSRF**，Go 侧同样只从 header 取， **不要为了方便加 cookie 或查询串回落** —— 由 `TestAuthTokenNotReadFromQueryOrCookie` 锁住。
`clientid` 仍保留 header-or-query 两种来源（对齐 `StringUtils.equalsAny`），因为它不是凭证、只是标识。

免鉴权名单：`middleware.auth.excludes`，默认值前 11 条逐字对应原项目
`security.excludes`（`ruoyi-admin/src/main/resources/application.yml:100-113`）， 外加一条 **`/auth/**`** —— Java 侧
`AuthController` 整个类挂 `@SaIgnore` 免鉴权，Go 没有注解机制只能进名单。 **漏了它登录接口自己就要 token，谁也登不进来**
，而症状（登录返回 401）会让人去查密码而不是查这份名单。 由 `TestAuthExcludesCoverLoginEndpoints` 与
`TestAuthExcludesMatchJavaSecurityExcludes` 锁住。

#### `ip.go`：两个容易静默放宽白名单的坑

对应 `ServletUtils.getClientIP`（`:277-285`）与 `NetUtils.isMatchIpRule`（`:93-149`）。 规则优先级照抄： **精确相等 → 含 `/` 走
CIDR → 含 `*`/`?` 走 glob → false**。

1. **CIDR 必须显式对齐地址族**。Go 的 `net.IP` 会把 IPv4 规范化成 16 字节的 v4-mapped 形式 （`::ffff:1.2.3.4`），于是
   `net.IPNet.Contains` 对 `::/0` 这条 v6 规则会把 IPv4 地址也算进去 —— **一条本意只放行 IPv6 的规则会静默放行全世界**
   。Java 侧靠
   `networkBytes.length != currentBytes.length` 挡住，Go 侧先各自 `To4()` 再比。
2. **glob 不用 regexp**。Java 是 `rule.replace(".","\\.").replace("*",".*").replace("?",".")` 配
   `String.matches`（ **全串**匹配），而 Go 的 `regexp.MatchString` 是 **部分**匹配 —— 照搬那三次 replace 会让
   `192.168.1.*` 命中 `10.0.0.1#192.168.1.5`。要修就得自己补 `^$` 锚点， 一个容易漏、漏了就是静默放宽的口子。改为复用
   `path.go` 的 `matchSegment`（同一套带回溯双指针）。
   `TestIsMatchIPRuleGlobIsFullMatch` 同时断言「Java 正则 + 部分匹配」确实会误判前缀。

**这些头都是可伪造的**（任何客户端都能自己发 `X-Forwarded-For`）。仍然信任它们是因为本项目部署在 nginx 之后、由 nginx
覆写该头。进程直接暴露公网时 IP 白名单形同虚设 —— 这与原项目相同，不是本实现引入的。

#### 登录/登出不在本包

`POST /auth/login` `/auth/logout` 在 `internal/auth`（对应 `AuthController` + `SysLoginService` +
`PasswordAuthStrategy`）。几条与本包配套的要点：

- **密码错误计数的顺序是安全要求**：「查用户」必须早于「碰计数器」。否则攻击者能用任意 **不存在**的 用户名把某个真实账号刷到锁定（计数键按
  username 存，而「这个用户名存不存在」在锁定前是未知的）。 对齐 Java 的 `loadUserByUsername` 先行，由
  `TestLoginNonexistentUserDoesNotIncrementRetryCount` 锁住。
- 代价是 **用户枚举可观测**（「账号不存在」与「密码输入错误N次」文案不同）。这是原项目的行为， 本次对齐 ——
  改掉它要连前端提示一起改，且属安全加固而非迁移。
- **授权类型用切分后精确比对**，不用 Java 的 `StringUtils.contains` 子串匹配 （那让 `grantType=pass` 命中
  `"password,social"`）。
- 密码是 BCrypt `$2a$10$`（hutool 默认 10 轮），`golang.org/x/crypto/bcrypt` 完全兼容 ——
  `TestPasswordVerifiesJavaBCryptHash` **直接拿原项目种子数据的哈希验 `admin123`**， 这是唯一能真正证明跨语言兼容的验证方式（往返测试做不到：变体或
  cost 选错也能自洽地往返成功）。

### 10. 响应增强：阶段 3 再说

`web/advice/ResponseEnhancementAdvice.java` 是 `ResponseBodyAdvice<Object>`，每个 JSON 响应过一遍
`JsonValueEnhancer.enhance(body)`，是字典翻译 / `@Translation` 字段填充的钩子。 阶段 0 **不建空壳**，等阶段 3 字典管理落地时按需引入。

## 不属于本包的（注解驱动，非全局）

这些在 Java 里是 `@Aspect` 或按注解生效，Go 侧应做成 **按路由挂的中间件或 handler 内显式调用**，不要塞进全局链：

| 关注点               | 原 Java 位置                                                                               | 阶段                                               |
|----------------------|--------------------------------------------------------------------------------------------|----------------------------------------------------|
| `@Log` 操作日志      | `ruoyi-common-log/…/log/aspect/LogAspect.java`（发 `OperLogEvent` 异步落库）               | 阶段 3                                             |
| 登录日志             | **不是中间件**，`ruoyi-admin/…/web/service/SysLoginService.java` 里手动发 `LoginInfoEvent` | 阶段 3（阶段 1 只打日志，`sys_login_info` 表待建） |
| `@SaCheckPermission` | `ruoyi-common-satoken/…/satoken/core/service/SaPermissionImpl.java` + 注解处理             | 阶段 2（权限码校验，依赖 `sys_menu`/`sys_role`）   |
| `@RateLimiter`       | `ruoyi-common-redis/…/redis/aspectj/RateLimiterAspect.java`                                | 阶段 4+                                            |
| `@RepeatSubmit`      | `ruoyi-common-redis/…/redis/aspectj/RepeatSubmitAspect.java`                               | 阶段 4+                                            |
| `@Lock4j`            | `ruoyi-common-redis/…/redis/config/Lock4jConfig.java`                                      | 阶段 4+                                            |
| `@DataPermission`    | `ruoyi-common-mybatis/…/mybatis/interceptor/PlusDataPermissionInterceptor.java`            | 阶段 4.1                                           |

> `@SaCheckPermission` / `@SaCheckRole` 与本包的 `auth.go` 是 **两件事**，别混：
> 前者是 **按接口**的权限码校验（`@SaCheckPermission("system:user:list")`），归阶段 2 做成按路由挂的中间件；
> 后者是 **全局**的「你是谁」校验。Java 侧两者也是分开的 —— `SaInterceptor` 先跑登录校验那个 lambda，
> 之后才处理方法上的注解。阶段 2 要用的 `menuPermission` / `rolePermission` 字段已在
> `auth.LoginUser` 里留好（当前为空），从 Redis 会话里取，不必重新查库。

`@ApiEncrypt` 曾经列在这张表里，现已落地为本包的 `crypto.go`（见上方第 8 节）—— 它 **不适合**「按路由挂」：
`CryptoFilter` 虽然靠注解决定对谁生效，但 **请求解密这一步是全局的、且必须在 `RepeatableBody` 之前**
（order 是 `HIGHEST_PRECEDENCE`），否则 `AccessLog` 只能看到密文、脱敏失效、handler 还绑不到参数。 Go 侧因此挂进全局链，靠配置里的路径清单代替注解。
**响应加密**可以按路由， **请求解密**不行。

数据权限见 `MIGRATION.md` 阶段 4.1 —— Java 是 MyBatis 拦截器改写 SQL，Go 要在 repository 层用 GORM Scopes 手写。 本包只负责
**把当前用户的数据范围写进 context**，SQL 条件由 repository 层拼。 那一半已经就位：`auth.go` 校验通过后把
`*auth.LoginUser` 同时写进 `gin.Context` 与 request 的 `context.Context`，repository 层用
`middleware.UserFromContext(ctx)` 取（ **不必 import gin**）。当前 `LoginUser` 里的角色与数据范围字段为空， 待阶段 2 的
`sys_role` 落地后填充。

## 相关 yaml key 速查

除 sa-token 那份外，均在 `ruoyi-admin/src/main/resources/application.yml`。已确认 dev/prod 两个 profile **没有覆盖任何中间件相关
key**，看主文件就够。

| key                        | 行号                                                         | 作用                                        | 本项目对应 key                                   |
|----------------------------|--------------------------------------------------------------|---------------------------------------------|--------------------------------------------------|
| `security.excludes`        | 100-113                                                      | 鉴权免登名单                                | `middleware.auth.excludes`（另加 `/auth/**`）    |
| `xss.enabled`              | 190-192                                                      | XSS 过滤开关                                | **有意无对应项**（见上方 XSS 一节）              |
| `xss.excludeUrls`          | 193-196                                                      | XSS 跳过路径                                | `middleware.xss.excludeUrls`                     |
| `web.cors.*`               | **缺失**                                                     | CORS，走代码默认值                          | `middleware.cors.*`（Go 侧提到了 yaml）          |
| `spring.messages.basename` | 61-63                                                        | i18n 词条目录                               | 无 —— 词条编进 Go 源码，见 `pkg/i18n`            |
| `message.path`             | 223                                                          | 被 `handleIoException` 读来静默 SSE 断连    | 无 —— Go 直接判 `*net.OpError`                   |
| `api-decrypt.*`            | 148-158                                                      | `CryptoFilter` 开关与 RSA 密钥              | `middleware.apiEncrypt.*`                        |
| `sa-token.token-name`      | 91                                                           | token 的 header/param 名（`Authorization`） | `middleware.auth.header`                         |
| `sa-token.token-prefix`    | `common-satoken.yml`                                         | token 前缀（`Bearer`）                      | `middleware.auth.tokenPrefix`                    |
| `sa-token.jwt-secret-key`  | 97                                                           | JWT 签名密钥                                | `jwt.secret`                                     |
| `user.password.*`          | 38-43                                                        | 密码最大错误次数与锁定时长                  | `user.password.*`（键路径与原项目一致）          |
| `is-read-cookie: false`    | `ruoyi-common-satoken/src/main/resources/common-satoken.yml` | 关掉 cookie 认证                            | 同样只从 header 取，不加 cookie/查询串回落       |
| `sa-token.timeout`         | 未全局配置                                                   | token 绝对超时                              | 无 —— 取 `sys_client.timeout`，落 JWT 的 `exp`   |
| `sa-token.active-timeout`  | 未全局配置                                                   | token 空闲超时（滑动）                      | 无 —— 取 `sys_client.active_timeout`，落会话 TTL |

> `jwt.header` 曾是 token 头名的对应项，现已由 `middleware.auth.header` 承担 ——
> 所有中间件配置都收在 `middleware` 段下、一个中间件一个文件（见「配置怎么读到的」），
> `jwt` 段只留签发用的 `secret` / `expireMinutes`。

Go 侧还有几项 **原项目没有**的配置（都是本包相对 Java 的新增行为，前面各节已逐条说明原因）：
`middleware.cors.exposedHeaders`、`middleware.traceId.*`、`middleware.accessLog.skipPaths`、
`middleware.repeatableBody.maxBodySize`、`middleware.i18n.header`、
`middleware.apiEncrypt.requestUrls` / `responseUrls` / `maxBodySize`、
`middleware.auth.clientIdHeader`
（`apiEncrypt` 那两项是注解的替代物 —— Java 靠 `@ApiEncrypt` 反查，Go 无注解只能显式列路径）。

## 配置怎么读到的

结构体定义在 `pkg/config/middleware.go`（不在本包），import 方向是 `middleware → config`，
`pkg/config` 保持叶子包、不依赖 gin。两个常量 `TraceIDHeader` / `LocaleHeader` 的 **定义**也在那边 （CORS 的默认
`ExposedHeaders` 要用前者），本包只留别名。`DefaultAPIEncryptHeader` 同理。

加解密的 **原语**在 `pkg/encrypt`（对应 `EncryptUtils.java`），本包只做策略（谁该加密、失败回什么）。 分开是因为阶段 4+ 的
`@EncryptField` 字段级加密（Java 侧走 `MybatisEncryptInterceptor`）要复用同一套原语， 而那条路径与 HTTP 无关、不该 import
gin。

每个中间件两个构造函数，职责分开：

| 形式                 | 配置来源                      | 用途                     |
|----------------------|-------------------------------|--------------------------|
| `XSS()`              | `config.Get().Middleware.XSS` | 正常注册，走 yaml        |
| `XSSWithConfig(cfg)` | 调用方显式传入                | 测试、或要绕开全局配置时 |

`WithConfig` 版本 **只用传入的那份配置**，不回头调 `config.Get()` —— 否则测试和不走 `Load` 的调用方会直接 panic。
`APIEncryptWithConfig` 因此有自己的 `maxBodySize` 而不是借 `repeatableBody` 那个（两者读的东西也不同： 一个是密文、一个是明文，base64
后差约 4/3）。

因此 **`config.Load` 必须早于 `middleware.Register`**，否则 `config.Get()` 会 panic （刻意为之：启动期编排错误不该留到运行时才发现）。配置在
`r.Use(...)` 那一刻读一次并捕获进闭包，
`Get()` 不进每请求的路径。 `APIEncrypt` 还在这一刻把 RSA 私钥解析好捕获进闭包 —— 密钥格式错误会 panic
在启动期，而不是留到运行期表现为「所有加密接口都失败而其余正常」那种半死状态。

关于 `enabled` 开关：6 个中间件 **有意都没有**（注册即启用），`apiEncrypt` 是 **唯一的例外**， 理由见上方第 8 节「为什么这一个有
`enabled`」。要关掉其余任何一环，改 `register.go` 删那一行。

`middleware` 段只放在 `configs/application.yaml`， **不要**放进 `system.yaml` / `auth.yaml` ——
理由与「两个入口的中间件链必须保持一致」同源：拆进程后请求落到哪个进程不定， 在那里开分叉口子等于把那条约束变成可选项。
