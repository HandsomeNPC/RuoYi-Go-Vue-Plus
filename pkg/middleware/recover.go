// Package middleware 通用 Gin 中间件(CORS / 全局异常 / 请求日志 / TraceID / 鉴权)。
//
// 各中间件与原项目的对应关系、注册顺序及注意事项见本目录 README.md。
package middleware

import (
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// 兜底提示，对应 GlobalExceptionHandler 里 RuntimeException / Exception 两个分支。
//
// Java 分成「未知异常」和「系统异常」两句，是因为它能按异常类型区分；
// Go 的 panic 值没有这种层次，统一用一句。
const msgUnknownError = "发生未知异常，请联系管理员"

// Recover 全局异常中间件，对应原项目 6 个 @RestControllerAdvice 的合集：
//
//	web/handler/GlobalExceptionHandler.java        主体
//	satoken/handler/SaTokenExceptionHandler.java   401/403
//	mybatis/handler/MybatisExceptionHandler.java   唯一键冲突等
//	redis/handler/RedisExceptionHandler.java       锁失败
//	sms/handler/SmsExceptionHandler.java           短信
//	workflow/handler/FlowExceptionHandler.java     工作流
//
// 后四个依赖尚未迁移的模块，等对应阶段再往 render 里加分支。
//
// 它覆盖两条来源不同的错误路径：
//
//  1. panic —— Java 没有对等物（Spring 靠异常冒泡），Go 必须自己拦，
//     否则一个 nil 解引用会打挂整个进程，而不是只失败一个请求。
//  2. c.Error(err) —— handler 主动登记的错误，等价于 Java 里 throw。
//
// 与 gin.Recovery() 的区别：gin 只写 500 空响应，这里要输出 response.R
// 并对齐 Java 的业务码语义，所以不能直接复用。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				handlePanic(c, r)
			}
		}()

		c.Next()

		// panic 路径已在 defer 里响应完毕，这里只处理 handler 登记的错误。
		// 已写过响应则不再覆盖：handler 可能已经流式输出（SSE/文件下载），
		// 此时再写 body 会污染报文。
		if len(c.Errors) > 0 && !c.Writer.Written() {
			render(c, c.Errors.Last().Err)
		}
	}
}

// handlePanic 处理 panic。
//
// 单独拆出来是因为它比 render 多两件事：连接中断需静默、必须打 goroutine 栈。
func handlePanic(c *gin.Context, r any) {
	// 客户端主动断开（用户关页面、SSE 断连）会让写响应 panic。
	// 对应 GlobalExceptionHandler.handleIoException 屏蔽 SSE 中断的意图，
	// 但判断依据不同：Java 靠 URI 前缀匹配 message.path 配置，
	// Go 直接判错误类型，比配路径准，也不必等 SSE 模块迁移过来。
	if isBrokenPipe(r) {
		log.Printf("[recover] 请求地址'%s',连接中断: %v", c.Request.URL.Path, r)
		// 连接已断，写不进任何东西，只能终止。
		c.Abort()
		return
	}

	// 栈必须在这里取：出了 defer 作用域栈就展开了，事后拿不到现场。
	log.Printf("[recover] 请求地址'%s',发生未知异常, 错误编号: %s\npanic: %v\n%s",
		c.Request.URL.Path, newErrorID(), r, debug.Stack())

	// panic 一律当系统异常，不解析成业务错误：
	// 业务错误应当走 return err，panic 到这里说明是 bug。
	if !c.Writer.Written() {
		c.AbortWithStatusJSON(http.StatusOK, response.Fail(msgUnknownError))
		return
	}
	c.Abort()
}

// render 把 handler 登记的 error 转成统一响应。
//
// HTTP 状态码恒为 200，业务码放响应体 —— 对齐原项目：
// Java 侧这些 advice 返回 R<Void> 且未标 @ResponseStatus，
// 前端统一拦 body.code 判成败，改成真实 4xx/5xx 会让前端拦截器失效。
func render(c *gin.Context, err error) {
	var se *errs.ServiceError
	if errors.As(err, &se) {
		// Detail 只进日志不回前端，对齐 detailMessage 的用途。
		if se.Detail != "" {
			log.Printf("[error] 请求地址'%s',业务异常: %s | 明细: %s",
				c.Request.URL.Path, se.Msg, se.Detail)
		} else {
			log.Printf("[error] 请求地址'%s',业务异常: %s", c.Request.URL.Path, se.Msg)
		}

		// Code 为 0 表示未指定业务码，回落 500，
		// 对齐 Java `code != null ? R.fail(code,msg) : R.fail(msg)`。
		if se.Code == 0 {
			c.AbortWithStatusJSON(http.StatusOK, response.Fail(se.Msg))
			return
		}
		c.AbortWithStatusJSON(http.StatusOK, response.FailCode(se.Code, se.Msg))
		return
	}

	// 非业务错误一律兜底，不把原始 err 回给前端 —— 里面可能有
	// SQL 片段、内网地址、文件路径。前端只拿错误编号，细节查日志。
	errorID := newErrorID()
	log.Printf("[error] 请求地址'%s',发生系统异常, 错误编号: %s: %v",
		c.Request.URL.Path, errorID, err)
	c.AbortWithStatusJSON(http.StatusOK,
		response.Fail(fmt.Sprintf("%s [错误编号: %s]", msgUnknownError, errorID)))
}

// newErrorID 生成 8 位错误编号，对应 Java 的 RandomUtil.randomNumbers(8)。
//
// 用途是把前端看到的提示和服务端日志对应起来。这是原项目唯一的请求关联手段
// （全项目无 traceId，见 README「TraceID」一节）。
// TraceID 中间件落地后，这里应改用 traceId 以贯穿一次请求的全部日志。
func newErrorID() string {
	return fmt.Sprintf("%08d", rand.IntN(100000000))
}

// isBrokenPipe 判断 panic 是否由客户端断连引起。
//
// 这类 panic 不是 bug，打完整栈只会淹没日志。Windows 与 Unix 的
// 报错文案不同，两边都要匹配。
func isBrokenPipe(r any) bool {
	ne, ok := r.(*net.OpError)
	if !ok {
		return false
	}
	var se *os.SyscallError
	if !errors.As(ne.Err, &se) {
		return false
	}
	msg := strings.ToLower(se.Error())
	return strings.Contains(msg, "broken pipe") || // Unix
		strings.Contains(msg, "connection reset by peer") || // Unix
		strings.Contains(msg, "forcibly closed") // Windows
}
