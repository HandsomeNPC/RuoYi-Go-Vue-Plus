package service

import (
	"context"
	"errors"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// ErrClientNotFound 客户端不存在。
var ErrClientNotFound = errors.New("service: 客户端不存在")

// ClientService 客户端业务逻辑。
type ClientService struct{}

// ClientSvcApp 包级实例。
var ClientSvcApp = new(ClientService)

// QueryByClientID 按客户端标识查客户端（对应 Java queryByClientId），
// 不存在时返回 ErrClientNotFound，返回填充好 *List 字段的 VO。
func (s *ClientService) QueryByClientID(ctx context.Context, clientID string) (*vo.SysClientVo, error) {
	client, err := repository.NewClientRepository(database.DB()).SelectByClientID(ctx, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrClientNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	out := vo.Conv.ConvertToSysClientVo(client)
	s.fillRuleFields(out)
	return out, nil
}

// QueryByID 按主键查客户端（对应 Java queryById），
// 不存在时返回 ErrClientNotFound，返回填充好 *List 字段的 VO。
func (s *ClientService) QueryByID(ctx context.Context, id int64) (*vo.SysClientVo, error) {
	client, err := repository.NewClientRepository(database.DB()).SelectByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrClientNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	out := vo.Conv.ConvertToSysClientVo(client)
	s.fillRuleFields(out)
	return out, nil
}

// QueryPageList 按条件分页查客户端，返回填充好 *List 字段的 VO 分页结果。
func (s *ClientService) QueryPageList(ctx context.Context, q bo.SysClientQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysClientVo], error) {

	res, err := repository.NewClientRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysClientVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(s.toVoList(res.Rows), res.Total), nil
}

// toVoList 批量转 VO 并回填规则字段。
func (s *ClientService) toVoList(clients []*model.SysClient) []*vo.SysClientVo {
	out := vo.Conv.ConvertToSysClientVoList(clients)
	for _, c := range out {
		s.fillRuleFields(c)
	}
	return out
}

// fillRuleFields 回填 VO 的扩展规则字段，便于前端直接展示/编辑
// （对应 Java SysClientServiceImpl#fillClientRuleFields）。
func (s *ClientService) fillRuleFields(c *vo.SysClientVo) {
	if c == nil {
		return
	}
	c.GrantTypeList = splitRules(c.GrantType)
	c.AccessPathList = parseAccessPathList(c.AccessPath)
	c.IPWhitelistList = splitRules(c.IPWhitelist)
}

// parseAccessPathList 切分访问路径串并对每条归一化。
func parseAccessPathList(s string) []string {
	rules := splitRules(s)
	for i, r := range rules {
		rules[i] = normalizeAccessPath(r)
	}
	return rules
}

// normalizeAccessPath 归一化单条访问路径规则：补前导斜杠，* 与 /** 统一成全放行。
func normalizeAccessPath(path string) string {
	if path == "*" || path == "/**" {
		return "/**"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// splitRules 按 , ; CR LF 切分规则串，trim 并丢弃空段（连续分隔符合并）。
func splitRules(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\r' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
