package push

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/config"
)

// bearerPrefix query token 里可能带的授权前缀（大小写不敏感）。
const bearerPrefix = "bearer "

// NormalizeQueryToken 把 query 里 `?Authorization=Bearer xxx` 的前缀剥掉，
// 使其与 header 形态一致。须注册在 TokenInterceptor 之前。
//
// 为什么必须有这一层：SSE 的 EventSource 与浏览器 WebSocket 都不能自定义请求头，
// 前端只能把 token 塞进 query（见 apps/web-antd/src/utils/message.ts）。而
// sa-token-go 的 ReadTokenFromRequest 只在 header 分支调 extractBearerToken，
// query 分支仅做 CutTokenPrefix —— 后者在未配置 TokenPrefix 时原样返回，
// 于是拿着 "Bearer eyJ..." 整串去查会话，必然查不到而 401。
//
// 改全局 TokenPrefix 也能让 CutTokenPrefix 生效，但那会连带要求所有普通接口的
// header 都带前缀，牵动面太大；只在推送这条链路上做规范化更克制。
func NormalizeQueryToken() gin.HandlerFunc {
	tokenName := config.Get().SAToken.TokenName

	return func(c *gin.Context) {
		raw := c.Request.URL.RawQuery
		// 快速排除：绝大多数请求不带 query token，避免逐个请求解析 query。
		// 只按 "bearer" 判断而不含空格——RawQuery 里空格是 %20 或 +，
		// 拿 "bearer " 去匹配编码后的串永远不命中。
		if raw == "" || !strings.Contains(strings.ToLower(raw), "bearer") {
			c.Next()
			return
		}

		values, err := url.ParseQuery(raw)
		if err != nil {
			c.Next()
			return
		}
		got := values.Get(tokenName)
		if len(got) <= len(bearerPrefix) ||
			!strings.EqualFold(got[:len(bearerPrefix)], bearerPrefix) {
			c.Next()
			return
		}

		values.Set(tokenName, strings.TrimSpace(got[len(bearerPrefix):]))
		c.Request.URL.RawQuery = values.Encode()
		// 不必清 gin 的 query 缓存：它是懒构建的（initQueryCache 在首次
		// c.Query 时才从 RawQuery 取值），而本中间件跑在 TokenInterceptor
		// 之前，此刻还没有人读过 query。但正因如此，本中间件必须排在所有
		// 读 query 的中间件之前，否则改动对已缓存的值无效。

		c.Next()
	}
}
