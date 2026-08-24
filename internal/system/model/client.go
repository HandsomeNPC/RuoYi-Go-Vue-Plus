package model

import (
	"strings"
	"time"
)

// SysClient 系统授权表，对应原项目 org.dromara.system.domain.SysClient 与表 sys_client。
//
// 表结构见原项目 script/sql/ry_vue.sql:847-869。
//
// # clientId 是什么
//
// `clientId = MD5(clientKey + clientSecret)`（SysClientServiceImpl.java:131），
// 例如 md5("pc"+"pc123") = e5cd7e4891bf95d1d19206ce24a7b32e。
//
// 它是**公开标识不是密钥** —— 前端每个请求都在 clientid 头里带着它。
// 用途是把 token 绑定到签发它的客户端，防止 app 端的 token 被拿去访问
// pc 端接口。真正的密钥是 clientSecret，只在登录时用。
type SysClient struct {
	ID        int64  `gorm:"column:id;primaryKey" json:"id"`
	ClientID  string `gorm:"column:client_id" json:"clientId"`
	ClientKey string `gorm:"column:client_key" json:"clientKey"`
	// ClientSecret 客户端密钥。json:"-" 同 SysUser.Password 的理由 ——
	// 它不该出现在任何响应体里。
	ClientSecret string `gorm:"column:client_secret" json:"-"`

	// GrantType 授权类型，逗号分隔的多值串（如 "password,social"）。
	// 取值域见 enum.LoginType 的 Code。用 GrantTypeList() 取切分后的列表。
	GrantType string `gorm:"column:grant_type" json:"grantType"`
	// DeviceType 设备类型，**自由字符串不是枚举** —— 种子数据里 app 客户端
	// 是 "android"，不在 enum.DeviceType 的取值域内（详见那个类型的注释）。
	DeviceType string `gorm:"column:device_type" json:"deviceType"`

	// AccessPath 允许访问的路径规则，Ant 风格，按 [,;\r\n]+ 分隔。
	// 空表示不限制。用 AccessPathList() 取切分并归一化后的列表。
	AccessPath string `gorm:"column:access_path" json:"accessPath"`
	// IPWhitelist 允许的来源 IP 规则（精确/CIDR/glob），同上分隔。空表示不限制。
	IPWhitelist string `gorm:"column:ip_whitelist" json:"ipWhitelist"`

	// ActiveTimeout token 活跃超时（秒），默认 1800。
	// 这是**滑动**的空闲超时：每请求重置，空闲超过它即失效。
	ActiveTimeout int64 `gorm:"column:active_timeout" json:"activeTimeout"`
	// Timeout token 固定超时（秒），默认 604800（7 天）。
	// 这是**绝对**有效期，落到 JWT 的 exp，到期无论多活跃都要重新登录。
	Timeout int64 `gorm:"column:timeout" json:"timeout"`

	// Status 状态（0正常 1停用）。
	Status  string `gorm:"column:status" json:"status"`
	DelFlag string `gorm:"column:del_flag" json:"-"`

	CreateDept int64      `gorm:"column:create_dept" json:"createDept"`
	CreateBy   int64      `gorm:"column:create_by" json:"createBy"`
	CreateTime *time.Time `gorm:"column:create_time" json:"createTime"`
	UpdateBy   int64      `gorm:"column:update_by" json:"updateBy"`
	UpdateTime *time.Time `gorm:"column:update_time" json:"updateTime"`
}

// TableName 显式指定表名，理由同 SysUser.TableName。
func (SysClient) TableName() string {
	return "sys_client"
}

// GrantTypeList 返回切分后的授权类型列表。
//
// 对应 SysClientVo.grantTypeList（由 fillClientRuleFields 填充）。
func (c *SysClient) GrantTypeList() []string {
	return splitRules(c.GrantType)
}

// SupportsGrantType 判断该客户端是否支持指定授权类型。
//
// **有意不照搬 Java 的实现**。那边是
// `StringUtils.contains(client.getGrantType(), grantType)`
// （AuthController.java:86）—— 对逗号拼接串做**子串**匹配，
// 于是 grantType="pass" 会命中 "password,social"、"word" 也会。
// 那是 bug 不是可对齐的行为：它让一个拼错或恶意构造的 grantType
// 通过校验，随后在策略分派处才失败（或更糟，命中了别的策略）。
//
// 这里改为切分后**精确比对**。差异只在畸形输入上 —— 正常前端发的
// 是完整的 "password"，两种实现结果相同。
func (c *SysClient) SupportsGrantType(grantType string) bool {
	if grantType == "" {
		return false
	}
	for _, t := range c.GrantTypeList() {
		if t == grantType {
			return true
		}
	}
	return false
}

// AccessPathList 返回切分并归一化后的访问路径规则。
//
// 对应 SysClientVo.accessPathList，归一化逻辑见 normalizeAccessPath。
func (c *SysClient) AccessPathList() []string {
	rules := splitRules(c.AccessPath)
	for i, r := range rules {
		rules[i] = normalizeAccessPath(r)
	}
	return rules
}

// IPWhitelistList 返回切分后的 IP 白名单规则（不做归一化）。
//
// 对应 SysClientVo.ipWhitelistList —— Java 侧对这一项传的是
// UnaryOperator.identity()，即原样保留。
func (c *SysClient) IPWhitelistList() []string {
	return splitRules(c.IPWhitelist)
}

// normalizeAccessPath 归一化单条访问路径规则。
//
// 对应 SysClientServiceImpl.normalizeAccessPath（:254-266）：
//
//	"*" 或 "/**" -> "/**"（全放行）
//	其余不以 / 开头的补上前导 /（"app/**" -> "/app/**"）
//
// 补前导斜杠这件事不能省：Ant 匹配要求 pattern 与 path 同时以 / 开头
// 或同时不以 / 开头（见 pkg/middleware/path.go 的 AntPathMatch），
// 而请求路径必然以 / 开头 —— 规则漏了它就一条也匹配不上，
// 表现为「配了白名单反而全被拒」。
func normalizeAccessPath(path string) string {
	if path == "*" || path == "/**" {
		return "/**"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// splitRules 按 , ; CR LF 切分规则串，trim 并丢弃空段。
//
// 对应 Java 的 CLIENT_RULE_SEPARATOR_REGEX "[,;\r\n]+" 配
// str2List(…, ignoreEmpty=true, isTrim=true)。
//
// 用 strings.FieldsFunc 而非 regexp：FieldsFunc 天然合并连续分隔符并
// 丢弃空段，与 "+" 量词加 ignoreEmpty 的行为一致，且不必编译正则。
//
// 全是分隔符或空白时返回 nil 而非空切片，与空入参那条路径一致 ——
// 两者对调用方等价（len 都是 0），但让返回值只有一种「空」的形态，
// 省得 reflect.DeepEqual 之类的比较在两个语义相同的值上给出不同答案。
//
// 注意 pkg/middleware/auth.go 里有一份同语义的 splitClientRules ——
// 那边切的是 JWT claims 里的规则串（鉴权热路径），这边切的是 DB 实体
// （登录时用一次）。**没有合并成一处**是因为合并意味着 middleware 要
// import internal/system/model，那会让 pkg 依赖 internal，破坏分层。
// 两者的一致性由各自的测试锁住。
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
