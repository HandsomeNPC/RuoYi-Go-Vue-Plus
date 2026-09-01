package push

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	sagin "github.com/sa-tokens/sa-token-go/integrations/gin"
	"github.com/sa-tokens/sa-token-go/storage/memory"

	"ruoyi-go-vue-plus/pkg/config"
	authmodel "ruoyi-go-vue-plus/pkg/model"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// TestSSEEndToEndWithQueryToken 用前端真实的 URL 形态跑通 SSE：
// ?Authorization=Bearer <jwt> 应得到 200 + text/event-stream，而不是 401。
func TestSSEEndToEndWithQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.Load("../../configs/application.yaml", "../../configs/system.yaml")
	// 与生产 satoken.Init 同构：JWT 风格 + 密钥，否则签出的 token 验不过。
	sagin.SetManager(sagin.NewBuilder().Storage(memory.NewStorage()).
		TokenName(config.Get().SAToken.TokenName).
		IsConcurrent(true).IsShare(false).
		TokenStyle(sagin.TokenStyleJWT).
		JwtSecretKey(config.Get().SAToken.JwtSecretKey).
		IsReadCookie(true).IsReadBody(true).Build())

	// 真实签发一个会话，拿到 token。
	lu := &authmodel.LoginUser{UserID: 1761100000000000001, UserType: "sys_user", Username: "admin"}
	token, err := loginhelper.Login(lu, "pc")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	// 手动装一个 SSE 管理器（不调 Init，避免依赖 Redis）。
	cfg := config.Get().Push
	cfg.Transport = config.PushTransportSSE
	m := &Manager{cfg: cfg, registry: newRegistry(), stop: make(chan struct{})}
	mu.Lock()
	defaultManager = m
	mu.Unlock()
	defer func() { mu.Lock(); defaultManager = nil; mu.Unlock() }()

	plugin := sagin.NewPlugin(sagin.GetManager())
	r := gin.New()
	r.GET("/resource/message", NormalizeQueryToken(), plugin.TokenInterceptor(),
		sagin.CheckLogin(), Handler())

	srv := httptest.NewServer(r)
	defer srv.Close()

	// 完全照抄用户 curl 的形态。
	url := srv.URL + "/resource/message?Authorization=Bearer%20" + token +
		"&clientid=e5cd7e4891bf95d1d19206ce24a7b32e"
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200（401 说明 query token 仍未被识别）", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, 期望 text/event-stream", ct)
	}
	t.Logf("状态=%d Content-Type=%q X-Accel-Buffering=%q",
		resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("X-Accel-Buffering"))

	// 应先收到 connected 注释行。
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("读首行失败: %v", err)
	}
	if !strings.Contains(line, "connected") {
		t.Errorf("首行 = %q, 期望含 connected", line)
	}
	t.Logf("收到首行: %q", line)

	if m.Online() != 1 {
		t.Errorf("在线连接数 = %d, 期望 1", m.Online())
	}
}

// TestSSERejectsBearerTokenWithoutNormalize 钉住 NormalizeQueryToken 的必要性：
// 摘掉它，同一个请求就会 401。
//
// 若哪天 sa-token-go 在 query 分支也调了 extractBearerToken，本用例会转为
// 失败——那正是该删掉中间件的信号，而不是"测试坏了"。
func TestSSERejectsBearerTokenWithoutNormalize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.Load("../../configs/application.yaml", "../../configs/system.yaml")
	sagin.SetManager(sagin.NewBuilder().Storage(memory.NewStorage()).
		TokenName(config.Get().SAToken.TokenName).
		IsConcurrent(true).IsShare(false).
		TokenStyle(sagin.TokenStyleJWT).
		JwtSecretKey(config.Get().SAToken.JwtSecretKey).
		IsReadCookie(true).IsReadBody(true).Build())

	lu := &authmodel.LoginUser{UserID: 1761100000000000001, UserType: "sys_user"}
	token, err := loginhelper.Login(lu, "pc")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	plugin := sagin.NewPlugin(sagin.GetManager())
	r := gin.New()
	// 有意不挂 NormalizeQueryToken。
	r.GET("/p", plugin.TokenInterceptor(), sagin.CheckLogin(),
		func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/p?Authorization=Bearer%20"+token, nil))

	if w.Code == http.StatusOK {
		t.Error("未规范化时带 Bearer 前缀的 query token 竟然通过了鉴权；" +
			"若上游库已支持，可移除 NormalizeQueryToken")
	}
}
