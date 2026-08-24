package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 载荷，对应原项目 sa-token 的 SaLoginParameter extra。
//
// # 为什么 JWT 和 Redis 会话各存一份
//
// 原项目用 StpLogicJwtForSimple（"simple 模式"）：token 本身是 JWT、携带身份与
// 8 个 extra，同时**仍把 LoginUser 整个存进 Redis**。两者分工：
//
//   - JWT claims：鉴权中间件每请求都要用的字段（clientid 交叉比对、
//     客户端访问规则）。放 claims 里就不必为了这几个值多读一次 Redis。
//   - Redis 会话：完整的 LoginUser（权限码、角色、岗位），只有业务需要时才读；
//     更重要的是它给了**撤销能力** —— JWT 自身签发后无法作废，
//     登出/踢下线靠删 Redis 键实现。
//
// 所以 claims 里的字段是「鉴权路径上要用的」，不是「登录用户的全部信息」。
// 想加字段前先问：鉴权中间件用得到吗？用不到就放 LoginUser。
//
// # 必须是具体结构体，不能用 jwt.MapClaims
//
// MapClaims 底层是 map[string]any，走 encoding/json 默认解码，数字一律解成
// float64 —— 只有 53 位有效位。而 UserID / DeptID 是 19 位雪花 id，
// 超出 2^53，尾数会被静默抹掉。
//
// 这与 xss.go / logger.go 里用 Decoder.UseNumber() 挡的是同一个坑，但后果更重：
// 那两处坏掉的是日志和请求体，这里坏掉的是**身份标识** —— 用一个被改过尾数的
// userId 去查库，查不到是好运气，查到别人是事故。
//
// 用 int64 字段则由 encoding/json 直接解进 int64，不经 float64 中转。
// 由 TestClaimsSnowflakeIDSurvivesRoundTrip 锁住。
type Claims struct {
	// RegisteredClaims 提供 sub（= LoginID）/ exp / iat。
	// exp 是**绝对超时**（对应 sys_client.timeout，默认 7 天）；
	// 滑动的空闲超时由 Redis 会话的 TTL 承担，见 session.go。
	jwt.RegisteredClaims

	// 以下 8 个对齐 Java LoginHelper 的 extra 键名（LoginHelper.java:37-45）。
	// json 标签必须逐字相同 —— 这是与前端和潜在的 Java 侧共存的协议。
	UserID       int64  `json:"userId"`
	Username     string `json:"userName"` // 注意是 userName 不是 username，对齐 USER_NAME_KEY
	DeptID       int64  `json:"deptId"`
	DeptName     string `json:"deptName"`
	DeptCategory string `json:"deptCategory"`

	// ClientID 客户端标识（= MD5(clientKey+clientSecret)），对应 CLIENT_KEY。
	// 鉴权时与请求头/查询串里的 clientid 交叉比对，防止 app 端签发的 token
	// 被拿去访问 pc 端接口。
	ClientID string `json:"clientid"`
	// ClientAccessPath 该客户端允许访问的路径规则（Ant 风格，逗号/分号/换行分隔）。
	// 空串表示不限制。
	ClientAccessPath string `json:"clientAccessPath"`
	// ClientIPWhitelist 该客户端允许的来源 IP 规则（精确/CIDR/glob，同上分隔）。
	// 空串表示不限制。
	ClientIPWhitelist string `json:"clientIpWhitelist"`
}
