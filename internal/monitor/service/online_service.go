// Package service 在线用户监控业务逻辑。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sa-tokens/sa-token-go/stputil"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/response"
)

// OnlineSvc 在线用户监控服务。
type OnlineSvc struct{}

var OnlineSvcApp = new(OnlineSvc)

// List 取所有未失效的在线会话，并按 IP/用户名过滤。
//
// 直接 Keys 扫描 online_tokens:* 而非走 sa-token 的 SearchTokenValue：在线摘要
// 是项目自管键，不在 sa-token 的 token-key 命名空间里。
func (s *OnlineSvc) List(ctx context.Context, ipaddr, userName string) ([]model.SysUserOnline, error) {
	keys, err := pkgredis.Client().Keys(ctx, constant.OnlineTokenKeyPrefix+"*").Result()
	if err != nil {
		return nil, errs.New(response.CodeFail, "获取在线用户列表失败", err.Error())
	}
	out := make([]model.SysUserOnline, 0, len(keys))
	for _, key := range keys {
		// key 形如 online_tokens:<token>，token 本身(JWT)不含 ":"，故直接去前缀。
		token := key[len(constant.OnlineTokenKeyPrefix):]
		uo, ok := s.fetchOnline(ctx, token)
		if !ok {
			continue
		}
		out = append(out, uo)
	}
	return filterOnline(out, ipaddr, userName), nil
}

// GetInfo 取指定账号仍有效的在线设备列表。loginID 形如 "sys_user:123"。
func (s *OnlineSvc) GetInfo(ctx context.Context, loginID string) ([]model.SysUserOnline, error) {
	tokens, err := stputil.GetTokenValueList(loginID)
	if err != nil {
		return nil, errs.New(response.CodeFail, "获取在线设备列表失败", err.Error())
	}
	out := make([]model.SysUserOnline, 0, len(tokens))
	for _, token := range tokens {
		uo, ok := s.fetchOnline(ctx, token)
		if !ok {
			continue
		}
		out = append(out, uo)
	}
	return out, nil
}

// ForceLogout 强制指定 token 下线。踢一个已失效的 token 不报错。
// 删除 online_tokens 摘要由 pkg/satoken 注册的 EventKickout 监听器兜住，此处不重复。
func (s *OnlineSvc) ForceLogout(_ context.Context, token string) error {
	if err := stputil.KickoutByToken(token); err != nil {
		log.Printf("[monitor] 踢出 token 失败(按已失效处理): %v", err)
	}
	return nil
}

// Remove 强退当前账号下指定设备：仅当 token 属于当前账号才踢，避免误伤他人会话。
func (s *OnlineSvc) Remove(ctx context.Context, loginID, token string) error {
	tokens, err := stputil.GetTokenValueList(loginID)
	if err != nil {
		return errs.New(response.CodeFail, "校验在线设备失败", err.Error())
	}
	for _, t := range tokens {
		if t == token {
			if err := stputil.KickoutByToken(token); err != nil {
				log.Printf("[monitor] 踢出自身 token 失败: %v", err)
			}
			break
		}
	}
	return nil
}

// fetchOnline 取单个 token 的在线摘要。返回 ok=false 表示该会话不应计入在线列表。
//
// 活跃超时已失效/被踢/被顶的 token 跳过：当前未配 active-timeout，
// CheckActiveTimeout 恒返回 nil，不过滤；一旦配上即自动生效。
// 缓存键缺失(令牌已过 TTL)或反序列化失败同样跳过。
func (s *OnlineSvc) fetchOnline(ctx context.Context, token string) (model.SysUserOnline, bool) {
	if err := stputil.CheckActiveTimeout(token); err != nil {
		return model.SysUserOnline{}, false
	}
	raw, err := pkgredis.Client().Get(ctx, constant.OnlineTokenKeyPrefix+token).Bytes()
	if err != nil {
		if !errors.Is(err, goredis.Nil) {
			log.Printf("[monitor] 读取在线摘要 token=%s 失败: %v", token, err)
		}
		return model.SysUserOnline{}, false
	}
	var uo model.SysUserOnline
	if err := json.Unmarshal(raw, &uo); err != nil {
		log.Printf("[monitor] 反序列化在线摘要 token=%s 失败: %v", token, err)
		return model.SysUserOnline{}, false
	}
	return uo, true
}

// filterOnline 按 IP/用户名过滤在线列表：
// 两者都给则同时匹配，只给其一则单匹配，都空则原样返回。
func filterOnline(in []model.SysUserOnline, ipaddr, userName string) []model.SysUserOnline {
	if ipaddr == "" && userName == "" {
		return in
	}
	out := make([]model.SysUserOnline, 0, len(in))
	for _, u := range in {
		var match bool
		switch {
		case ipaddr != "" && userName != "":
			match = u.IPAddr == ipaddr && u.UserName == userName
		case ipaddr != "":
			match = u.IPAddr == ipaddr
		default:
			match = u.UserName == userName
		}
		if match {
			out = append(out, u)
		}
	}
	return out
}
