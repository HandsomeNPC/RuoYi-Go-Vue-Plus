package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	sagin "github.com/HandsomeNPC/sa-token-go/integrations/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/middleware/accesslog"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/apiencrypt"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/cors"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/ratelimiter"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/repeatablebody"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/trace"
	"github.com/HandsomeNPC/ruoyi-common-go/middleware/xss"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/audit"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/checklogin"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/notfound"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/middleware/recover"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/satoken"
	authconfig "ruoyi-go-vue-plus/internal/auth/config"
	authhandler "ruoyi-go-vue-plus/internal/auth/handler/auth"
)

func RegisterRoutes(r *gin.Engine, prefix string) {
	plugin := sagin.NewPlugin(satoken.Manager())
	enc := apiencrypt.APIEncryptWithConfig(authconfig.Get().APIEncrypt)

	g := r.Group(prefix)
	// AuditContext 须排在 TokenInterceptor 之后。auth 现有接口写的 sys_login_info 无审计列，
	// 这里挂上是为了将来加注册/改密码等写接口时不会静默把 create_by 落成 -1。
	g.Use(plugin.TokenInterceptor(), audit.AuditContext())
	{
		g.POST("/login", sagin.Ignore(), enc, authhandler.Login)
		g.POST("/logout", sagin.Ignore(), authhandler.Logout)
		g.GET("/code", sagin.Ignore(),
			ratelimiter.RateLimiter(time.Minute, 10, ratelimiter.LimitTypeIP, 0, ""),
			authhandler.Code)
		// 三方绑定三件套。binding 公开：登录页未登录时点三方登录也要拿得到跳转地址。
		// 另两个需登录：绑定/解绑都是给当前登录用户操作的。
		g.GET("/binding/:source", sagin.Ignore(), authhandler.Binding)
		g.POST("/social/callback", checklogin.CheckLogin(), authhandler.SocialCallback)
		g.DELETE("/unlock/:socialId", checklogin.CheckLogin(), authhandler.UnlockSocial)
		g.GET("/ping", sagin.Ignore(), func(c *gin.Context) {
			c.JSON(http.StatusOK, response.Ok(gin.H{"module": "auth", "message": "pong"}))
		})
	}
}

// InitRouter 构建并返回 auth 进程的 gin 引擎(独立部署用)。
func InitRouter() *gin.Engine {
	r := gin.New()
	cfg := authconfig.Get()

	r.Use(recover.Recover())
	r.Use(cors.CORSWithConfig(cfg.CORS))
	r.Use(trace.TraceIDWithConfig(cfg.TraceID))
	r.Use(repeatablebody.RepeatableBodyWithConfig(cfg.RepeatableBody))
	r.Use(accesslog.AccessLogWithConfig(cfg.AccessLog))
	r.Use(xss.XSSWithConfig(cfg.XSS))

	RegisterRoutes(r, "")
	notfound.Register(r)

	return r
}
