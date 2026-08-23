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

| # | 关注点          | 原 Java 位置                                                                            | 本包文件            |
|---|-----------------|-----------------------------------------------------------------------------------------|---------------------|
| 1 | 全局异常        | `ruoyi-common-web/…/web/handler/GlobalExceptionHandler.java` + 另 5 个 advice（见下）   | ✅ `recover.go`     |
| 2 | CORS            | `ruoyi-common-web/…/web/config/ResourcesConfig.java:73-86`（`CorsFilter` bean）         | ✅ `cors.go`        |
| 3 | TraceID         | **原项目不存在，净新增**                                                                | ✅ `trace.go`       |
| 4 | 可重复读 Body   | `ruoyi-common-web/…/web/filter/RepeatableFilter.java` + `RepeatedlyRequestWrapper.java` | ✅ `body.go`        |
| 5 | 请求日志 + 耗时 | `ruoyi-common-web/…/web/interceptor/PlusWebInvokeTimeInterceptor.java`                  | ✅ `logger.go`      |
| 6 | XSS 过滤        | `ruoyi-common-web/…/web/filter/XssFilter.java` + `XssHttpServletRequestWrapper.java`    | ✅ `xss.go`         |
| 7 | i18n            | `ruoyi-common-web/…/web/config/I18nConfig.java` + `web/core/I18nLocaleResolver.java`    | ✅ `i18n.go`        |
| 8 | 鉴权            | `ruoyi-common-security/…/security/config/SecurityConfig.java:80-119`                    | `auth.go`（阶段 1） |
| 9 | 响应增强        | `ruoyi-common-web/…/web/advice/ResponseEnhancementAdvice.java`                          | 阶段 3 再建         |

另有两个没有 Java 对照物的文件：`register.go`（按序注册全部中间件，对应 Spring 的四个 `AutoConfiguration.imports`
清单）与 `path.go`（Ant 路径匹配，对应 `AntPathMatcher`，被 `xss.go` 与阶段 1 的 `auth.go` 共用）。 配置结构体不在本包，在
`pkg/config/middleware.go`（见下方「配置怎么读到的」）。

### 注册顺序

```
Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS → I18n → Auth
```

三个顺序约束不能动：

- **CORS 必须在 Auth 之前**，否则浏览器 preflight（`OPTIONS`，不带 token）会被 401，前端拿不到跨域头。
- **RepeatableBody 必须在 AccessLog 之前**，Java 侧 `PlusWebInvokeTimeInterceptor` 只在请求是
  `RepeatedlyRequestWrapper` 时才读 body（源码里显式判类型），Go 里同理：body 是一次性 `io.ReadCloser`，日志读完 handler
  就绑不到参数了。
- **XSS 必须在所有「读 gin 参数」的中间件之前**（同时也必须在 `RepeatableBody` 之后）。gin 的 `c.Query` 会把
  `URL.Query()` 的结果缓存进内部 `queryCache`，缓存一旦建立， XSS 再改 `URL.RawQuery` 就 **静默失效**。当前链路里 XSS
  之前没有任何一环读 gin 参数（`AccessLog` 走 `c.Request.ParseForm`，不碰 gin 的缓存；`I18n` 排在 XSS 之后且只读请求头）， 阶段
  1 的 `Auth` 读 `clientid` 排在 XSS 之后 —— 拿到的是清洗后的值。 这条约束由 `TestXSSIneffectiveIfQueryReadEarlier` 锁住：它
  **故意**把读参数的中间件
  前置并断言清洗失效，将来谁把鉴权挪到 XSS 前面，那条用例就会以「清洗突然生效了」的形式报错。

第四条约束来自 i18n，宽松得多： **I18n 必须在 Auth 之前**（鉴权的提示文案要走词条）。它不读 body、不改请求， 与前三条没有交集，详见下方
i18n 一节。

> 截至当前，`Recover → CORS → TraceID → RepeatableBody → AccessLog → XSS → I18n` 已全部在
> `register.go` 的 `Register(r)` 里注册，两个入口各一行调用；只剩 `Auth` 待阶段 1 落地。
> **两个入口的中间件链必须保持一致** —— 拆进程后同一个请求经 nginx 落到哪个进程是不定的，
> 两边链路不同会让同一次调用表现出不同的清洗/脱敏行为，而那种差异极难从现象反推。
> 收进 `Register` 就是为了让这件事 **物理上无法**做错：加 `Auth` 时只改一个文件，两个进程自动同步。

还有一条 **将来**才会用上、但现在就得记下的： **`ApiEncrypt` 解密中间件必须在 `RepeatableBody` 之前**， 即最终顺序为
`Recover → CORS → TraceID → ApiEncrypt → RepeatableBody → AccessLog → ...`。 依据是 Java 侧的 Filter
`order`（数值越小越先执行）：

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
- 放在 `RepeatableBody` 之前 → 日志是明文、脱敏正常生效，但这也意味着 **明文密码会流进 `jsonParamLog`**，
  `removeSensitiveFields` 那条路径必须靠得住。

`api-decrypt.enabled` 在 `application.yml:150` 是 **`true`**，前端现在就在加密部分请求，这不是假设问题。 当前未落地
`ApiEncrypt` 中间件，所以加密请求体会以 base64 密文原样进日志 —— 不构成泄漏（密文对读日志的人无用）， 但是纯噪音（最长 4000
字符的乱码，零诊断价值）。等 `ApiEncrypt` 落地、顺序摆对，这条自然消失。

> 未核实项：`SecureUtil.aes(byte[])` 用的是 `SymmetricAlgorithm.AES`，即 JCE 默认的 **AES/ECB/PKCS5Padding**（无 IV）。
> ECB 的确定性会不会让密文成为可比对的指纹，取决于前端是否每次请求都换 AES key —— 仓库内无前端代码，未能验证。

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
`Register` 没挂它」这种要翻两处才能确诊的状态。要关掉某一环，改 `register.go` 删那一行。 这条对 6 个中间件一律适用，不是 XSS
的特例。

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

### 8. 鉴权（阶段 1）：四步校验 + 一个易踩的坑

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

但 **解密这一步是全局的、且必须在 `RepeatableBody` 之前**（`CryptoFilter` 的 order 是 `HIGHEST_PRECEDENCE`）， 否则
`AccessLog` 只能看到密文、脱敏失效、handler 还绑不到参数。详见上方「注册顺序」一节 —— 那里有完整的 Filter order 对照表和
`DecryptRequestBodyWrapper` 恒返回 `application/json` 的依据。 这也是「按路由挂」的例外： **响应加密**可以按路由，
**请求解密**不行。

数据权限见 `MIGRATION.md` 阶段 4.1 —— Java 是 MyBatis 拦截器改写 SQL，Go 要在 repository 层用 GORM Scopes 手写。 本包只负责
**把当前用户的数据范围写进 context**，SQL 条件由 repository 层拼。

## 相关 yaml key 速查

除 sa-token 那份外，均在 `ruoyi-admin/src/main/resources/application.yml`。已确认 dev/prod 两个 profile **没有覆盖任何中间件相关
key**，看主文件就够。

| key                        | 行号                                                         | 作用                                        | 本项目对应 key                                 |
|----------------------------|--------------------------------------------------------------|---------------------------------------------|------------------------------------------------|
| `security.excludes`        | 100-113                                                      | 鉴权免登名单                                | 阶段 1 落地                                    |
| `xss.enabled`              | 190-192                                                      | XSS 过滤开关                                | **有意无对应项**（见上方 XSS 一节）            |
| `xss.excludeUrls`          | 193-196                                                      | XSS 跳过路径                                | `middleware.xss.excludeUrls`                   |
| `web.cors.*`               | **缺失**                                                     | CORS，走代码默认值                          | `middleware.cors.*`（Go 侧提到了 yaml）        |
| `spring.messages.basename` | 61-63                                                        | i18n 词条目录                               | 无 —— 词条编进 Go 源码，见 `pkg/i18n`          |
| `message.path`             | 223                                                          | 被 `handleIoException` 读来静默 SSE 断连    | 无 —— Go 直接判 `*net.OpError`                 |
| `api-decrypt.*`            | 148-158                                                      | `CryptoFilter` 开关与 RSA 密钥              | 按需                                           |
| `sa-token.token-name`      | 91                                                           | token 的 header/param 名（`Authorization`） | `jwt.header`                                   |
| `sa-token.jwt-secret-key`  | 97                                                           | JWT 签名密钥                                | `jwt.secret`                                   |
| `is-read-cookie: false`    | `ruoyi-common-satoken/src/main/resources/common-satoken.yml` | 关掉 cookie 认证                            | 阶段 1 —— 同样只从 header 取，不加 cookie 回落 |

Go 侧还有几项 **原项目没有**的配置（都是本包相对 Java 的新增行为，前面各节已逐条说明原因）：
`middleware.cors.exposedHeaders`、`middleware.traceId.*`、`middleware.accessLog.skipPaths`、
`middleware.repeatableBody.maxBodySize`、`middleware.i18n.header`。

## 配置怎么读到的

结构体定义在 `pkg/config/middleware.go`（不在本包），import 方向是 `middleware → config`，
`pkg/config` 保持叶子包、不依赖 gin。两个常量 `TraceIDHeader` / `LocaleHeader` 的 **定义**也在那边 （CORS 的默认
`ExposedHeaders` 要用前者），本包只留别名。

每个中间件两个构造函数，职责分开：

| 形式                 | 配置来源                      | 用途                     |
|----------------------|-------------------------------|--------------------------|
| `XSS()`              | `config.Get().Middleware.XSS` | 正常注册，走 yaml        |
| `XSSWithConfig(cfg)` | 调用方显式传入                | 测试、或要绕开全局配置时 |

因此 **`config.Load` 必须早于 `middleware.Register`**，否则 `config.Get()` 会 panic （刻意为之：启动期编排错误不该留到运行时才发现）。配置在
`r.Use(...)` 那一刻读一次并捕获进闭包，
`Get()` 不进每请求的路径。

`middleware` 段只放在 `configs/application.yaml`， **不要**放进 `system.yaml` / `auth.yaml` ——
理由与「两个入口的中间件链必须保持一致」同源：拆进程后请求落到哪个进程不定， 在那里开分叉口子等于把那条约束变成可选项。
