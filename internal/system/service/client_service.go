package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strings"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// ErrClientNotFound 客户端不存在。
var ErrClientNotFound = errors.New("service: 客户端不存在")

// ErrClientKeyExists 客户端 key 已被占用。
var ErrClientKeyExists = errors.New("service: 客户端key已存在")

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

// CheckClientKeyUnique 校验 client_key 是否可用（对齐 Java checkClickKeyUnique，同为「唯一即 true」）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *ClientService) CheckClientKeyUnique(ctx context.Context, clientKey string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewClientRepository(database.DB()).
		ExistsByClientKey(ctx, clientKey, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// InsertByBo 新增客户端（对应 Java insertByBo）。
// client_key 重复时返回 ErrClientKeyExists；插入成功后回填 b.ID。
func (s *ClientService) InsertByBo(ctx context.Context, b *bo.SysClientBo) error {
	if b == nil {
		return errors.New("service: 客户端入参为空")
	}

	unique, err := s.CheckClientKeyUnique(ctx, b.ClientKey, 0) // 新增无自身可排除
	if err != nil {
		return err
	}
	if !unique {
		return ErrClientKeyExists
	}

	add := bo.Conv.ConvertToSysClient(b)
	add.ID = snowflake.Next() // id 无 auto_increment
	add.ClientID = newClientID(b.ClientKey, b.ClientSecret)
	// 授权类型只做拼接，不切分也不归一（对齐 Java CollUtil.join）。
	add.GrantType = strings.Join(b.GrantTypeList, ",")
	add.AccessPath = resolveRuleValue(b.AccessPath, b.AccessPathList, normalizeAccessPath)
	add.IPWhitelist = resolveRuleValue(b.IPWhitelist, b.IPWhitelistList, nil)
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewClientRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.ID = add.ID
	return nil
}

// UpdateByBo 修改客户端（对应 Java updateByBo）。
// client_key 被别的客户端占用时返回 ErrClientKeyExists；主键不存在返回 ErrClientNotFound。
func (s *ClientService) UpdateByBo(ctx context.Context, b *bo.SysClientBo) error {
	if b == nil {
		return errors.New("service: 客户端入参为空")
	}
	if b.ID <= 0 {
		return errors.New("service: 客户端主键不能为空")
	}

	unique, err := s.CheckClientKeyUnique(ctx, b.ClientKey, b.ID) // 排除自身，改回原 key 不算冲突
	if err != nil {
		return err
	}
	if !unique {
		return ErrClientKeyExists
	}

	// 一律写入，让前端能把访问路径/IP 白名单清空——这正是编辑表单的本意，
	// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
	columns := buildClientUpdateColumns(b)

	affected, err := repository.NewClientRepository(database.DB()).UpdateByID(ctx, b.ID, columns)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrClientNotFound
	}
	return nil
}

// buildClientUpdateColumns 组装修改客户端的更新列。
func buildClientUpdateColumns(b *bo.SysClientBo) map[string]any {
	columns := map[string]any{
		"client_key":    b.ClientKey,
		"client_secret": b.ClientSecret,
		"grant_type":    strings.Join(b.GrantTypeList, ","),
		"device_type":   b.DeviceType,
		"access_path":   resolveRuleValue(b.AccessPath, b.AccessPathList, normalizeAccessPath),
		"ip_whitelist":  resolveRuleValue(b.IPWhitelist, b.IPWhitelistList, nil),
		// client_id 跟着 key/secret 重算——与 Java updateByBo 的有意差异：
		// 它沿用前端回填的 clientId，改了 key/secret 后该值即失真，
		// 而 auth 登录正是按 client_id 查库，会静默查不到客户端。
		"client_id": newClientID(b.ClientKey, b.ClientSecret),
	}
	// 状态与两个超时缺省即视为不改：漏传字段不该把线上 status 刷成空串、
	// 或把超时刷成 0 令已签发的 token 立刻失效。等效于 Java 的 null 跳过。
	if b.Status != "" {
		columns["status"] = b.Status
	}
	if b.ActiveTimeout != 0 {
		columns["active_timeout"] = b.ActiveTimeout
	}
	if b.Timeout != 0 {
		columns["timeout"] = b.Timeout
	}
	return columns
}

// UpdateClientStatus 改客户端启停状态（对应 Java updateClientStatus，同为按 client_id 定位）。
// 客户端不存在时返回 ErrClientNotFound。
func (s *ClientService) UpdateClientStatus(ctx context.Context, clientID, status string) error {
	if clientID == "" {
		return errors.New("service: 客户端标识不能为空")
	}

	affected, err := repository.NewClientRepository(database.DB()).
		UpdateStatusByClientID(ctx, clientID, status)
	if err != nil {
		return err
	}
	// 状态未变时 MySQL 也报 0 行；此处按 Java toAjax(0) 的口径一并当作失败回报。
	if affected == 0 {
		return ErrClientNotFound
	}
	return nil
}

// DeleteWithValidByIDs 批量删除客户端（对应 Java deleteWithValidByIds）。
// 无一行命中时返回 ErrClientNotFound，对齐 Java deleteByIds() > 0 的失败口径。
func (s *ClientService) DeleteWithValidByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 客户端主键不能为空")
	}

	affected, err := repository.NewClientRepository(database.DB()).DeleteByIDs(ctx, ids)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrClientNotFound
	}
	return nil
}

// newClientID 生成客户端标识 md5(clientKey + clientSecret)。
// md5 由 Java SecureUtil.md5 对齐所迫；该值是客户端查找键，不是机密。
func newClientID(clientKey, clientSecret string) string {
	sum := md5.Sum([]byte(clientKey + clientSecret))
	return hex.EncodeToString(sum[:])
}

// resolveRuleValue 归一化规则串的入库格式（对应 Java resolveRuleValue）：
// raw 非空时切分 raw，否则用 list（list 不再切分，只归一化），结果以逗号拼接。
//
// 与 Java 的一处有意差异：Java 靠 rawValue == null 区分「字段缺省」与「显式传空串」，
// Go 的 BO 字段是 string、两者都塌成 ""，故以 raw != "" 作代理。
// access_path/ip_whitelist 均为 default null 且回读时 splitRules("") == nil，
// 故落库 "" 与 NULL 透过 API 观察不可区分。
func resolveRuleValue(raw string, list []string, normalize func(string) string) string {
	rules := list
	if raw != "" {
		rules = splitRules(raw)
	}

	out := make([]string, 0, len(rules))
	for _, r := range rules {
		if r = strings.TrimSpace(r); r == "" {
			continue
		}
		if normalize != nil {
			if r = normalize(r); r == "" {
				continue
			}
		}
		out = append(out, r)
	}
	return strings.Join(out, ",")
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
