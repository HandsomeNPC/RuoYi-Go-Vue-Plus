package repository

import (
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// ErrInvalidOrderBy 排序参数非法，用 ServiceError 以便中间件渲染成 400。
var ErrInvalidOrderBy = errs.New(response.CodeBadRequest, "排序参数有误", "orderByColumn/isAsc 不合法")

const (
	DefaultPageNum  = 1   // 默认页码
	DefaultPageSize = 10  // 默认每页条数，这里收敛成 10 当 LIMIT 下发
	MaxPageSize     = 500 // 每页条数上限，防止前端传天文数字拖垮数据库
)

// orderByPattern 排序列名白名单，防排序注入。
// 列名无法参数化只能拼进 SQL，这里是防注入的唯一防线。
var orderByPattern = regexp.MustCompile(`^[a-zA-Z0-9_ ,.]+$`)

// PageQuery 分页查询入参，由 handler 用 ShouldBindQuery 绑定。
type PageQuery struct {
	PageNum       int    `form:"pageNum" json:"pageNum"`             // 当前页码，从 1 开始
	PageSize      int    `form:"pageSize" json:"pageSize"`           // 每页条数
	OrderByColumn string `form:"orderByColumn" json:"orderByColumn"` // 排序列，多列逗号分隔，支持驼峰
	IsAsc         string `form:"isAsc" json:"isAsc"`                 // asc/desc，兼容 ascending/descending
}

// NewPageQuery 构造分页查询对象。
func NewPageQuery(pageNum, pageSize int) PageQuery {
	return PageQuery{PageNum: pageNum, PageSize: pageSize}
}

// Num 归一化页码，非法值回落默认。
func (q PageQuery) Num() int {
	if q.PageNum <= 0 {
		return DefaultPageNum
	}
	return q.PageNum
}

// Size 归一化页大小，非法回落默认，超限截断到 MaxPageSize。
func (q PageQuery) Size() int {
	if q.PageSize <= 0 {
		return DefaultPageSize
	}
	if q.PageSize > MaxPageSize {
		return MaxPageSize
	}
	return q.PageSize
}

// Offset 当前页起始行号。
func (q PageQuery) Offset() int {
	return (q.Num() - 1) * q.Size()
}

// Paginate 返回只做 LIMIT/OFFSET 的 GORM Scope，不含排序（Count 不需要排序）。
func (q PageQuery) Paginate() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(q.Offset()).Limit(q.Size())
	}
}

// HasOrder 是否指定了排序。两个参数缺任一即视为未指定，OrderBy 据此返回零值，
// repository 据此决定是否补默认排序——判定只此一处，避免两侧漂移。
func (q PageQuery) HasOrder() bool {
	return strings.TrimSpace(q.OrderByColumn) != "" && strings.TrimSpace(q.IsAsc) != ""
}

// OrderBy 构建排序子句。无排序参数时返回零值（Columns 为空，调用方据此跳过）；
// 参数非法（含 SQL 关键字、方向词拼错、列数与方向数不匹配）时返回 error。
func (q PageQuery) OrderBy() (clause.OrderBy, error) {
	if !q.HasOrder() {
		return clause.OrderBy{}, nil
	}
	if !orderByPattern.MatchString(q.OrderByColumn) {
		return clause.OrderBy{}, ErrInvalidOrderBy
	}

	direction := strings.NewReplacer("ascending", "asc", "descending", "desc").Replace(q.IsAsc)
	columns := splitAndTrim(q.OrderByColumn)
	directions := splitAndTrim(direction)
	if len(columns) == 0 || len(directions) == 0 {
		return clause.OrderBy{}, ErrInvalidOrderBy
	}
	// 方向要么 1 个（作用于全部列），要么与列数一一对应。
	if len(directions) != 1 && len(directions) != len(columns) {
		return clause.OrderBy{}, ErrInvalidOrderBy
	}

	items := make([]clause.OrderByColumn, 0, len(columns))
	for i, col := range columns {
		dir := directions[0]
		if len(directions) > 1 {
			dir = directions[i]
		}
		var desc bool
		switch strings.ToLower(dir) {
		case "asc":
			desc = false
		case "desc":
			desc = true
		default:
			return clause.OrderBy{}, ErrInvalidOrderBy
		}
		items = append(items, clause.OrderByColumn{
			Column: clause.Column{Name: toUnderScoreCase(col)}, // Raw=false，dialector 自动加反引号
			Desc:   desc,
		})
	}
	return clause.OrderBy{Columns: items}, nil
}

// splitAndTrim 按逗号切分并去空白项。
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// toUnderScoreCase 驼峰转下划线：createTime -> create_time、userID -> user_id。
// 连续大写视作缩写不拆，已是下划线原样返回。
func toUnderScoreCase(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if r < 'A' || r > 'Z' {
			b.WriteRune(r)
			continue
		}
		if i > 0 && !isUpper(runes[i-1]) && runes[i-1] != '_' {
			b.WriteByte('_')
		}
		b.WriteRune(r - 'A' + 'a')
	}
	return b.String()
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }
