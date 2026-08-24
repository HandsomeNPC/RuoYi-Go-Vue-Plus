// Package middleware 通用 Gin 中间件。
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

// msgUnknownError 兜底提示。
const msgUnknownError = "发生未知异常，请联系管理员"

// Recover 全局异常中间件，覆盖 panic 与 handler 登记的 c.Error 两类错误。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				handlePanic(c, r)
			}
		}()

		c.Next()

		// panic 路径已在 defer 里响应完毕，这里只处理 handler 登记的错误。
		// 已写过响应则不再覆盖。
		if len(c.Errors) > 0 && !c.Writer.Written() {
			render(c, c.Errors.Last().Err)
		}
	}
}

// handlePanic 处理 panic。
func handlePanic(c *gin.Context, r any) {
	// 客户端主动断开会让写响应 panic，对齐 SSE 中断的屏蔽意图。
	if isBrokenPipe(r) {
		log.Printf("[recover]%s 请求地址'%s',连接中断: %v",
			logTracePrefix(c), c.Request.URL.Path, r)
		c.Abort()
		return
	}

	// 栈必须在这里取：出了 defer 作用域栈就展开了。
	log.Printf("[recover]%s 请求地址'%s',发生未知异常, 错误编号: %s\npanic: %v\n%s",
		logTracePrefix(c), c.Request.URL.Path, newErrorID(), r, debug.Stack())

	// panic 一律当系统异常。
	if !c.Writer.Written() {
		c.AbortWithStatusJSON(http.StatusOK, response.Fail(msgUnknownError))
		return
	}
	c.Abort()
}

// render 把 handler 登记的 error 转成统一响应。HTTP 状态码恒为 200。
func render(c *gin.Context, err error) {
	var se *errs.ServiceError
	if errors.As(err, &se) {
		// Detail 只进日志不回前端。
		if se.Detail != "" {
			log.Printf("[error]%s 请求地址'%s',业务异常: %s | 明细: %s",
				logTracePrefix(c), c.Request.URL.Path, se.Msg, se.Detail)
		} else {
			log.Printf("[error]%s 请求地址'%s',业务异常: %s",
				logTracePrefix(c), c.Request.URL.Path, se.Msg)
		}

		// Code 为 0 表示未指定业务码，回落 500。
		if se.Code == 0 {
			c.AbortWithStatusJSON(http.StatusOK, response.Fail(se.Msg))
			return
		}
		c.AbortWithStatusJSON(http.StatusOK, response.FailCode(se.Code, se.Msg))
		return
	}

	// 非业务错误一律兜底，不把原始 err 回给前端。
	errorID := newErrorID()
	log.Printf("[error]%s 请求地址'%s',发生系统异常, 错误编号: %s: %v",
		logTracePrefix(c), c.Request.URL.Path, errorID, err)
	c.AbortWithStatusJSON(http.StatusOK,
		response.Fail(fmt.Sprintf("%s [错误编号: %s]", msgUnknownError, errorID)))
}

// newErrorID 生成 8 位错误编号，用于把前端提示与服务端日志对应起来。
func newErrorID() string {
	return fmt.Sprintf("%08d", rand.IntN(100000000))
}

// isBrokenPipe 判断 panic 是否由客户端断连引起。Windows 与 Unix 文案不同，两边都要匹配。
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
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "forcibly closed")
}
