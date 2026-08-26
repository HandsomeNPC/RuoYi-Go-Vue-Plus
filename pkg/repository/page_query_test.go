package repository

import (
	"errors"
	"strings"
	"testing"
)

func TestNumSizeOffsetDefaults(t *testing.T) {
	tests := []struct {
		name              string
		q                 PageQuery
		wantNum, wantSize int
		wantOffset        int
	}{
		{"零值回落默认", PageQuery{}, DefaultPageNum, DefaultPageSize, 0},
		{"负页码回落", PageQuery{PageNum: -3, PageSize: 20}, 1, 20, 0},
		{"正常取值", PageQuery{PageNum: 3, PageSize: 20}, 3, 20, 40},
		{"超限截断", PageQuery{PageNum: 1, PageSize: 999999}, 1, MaxPageSize, 0},
		{"第二页偏移", PageQuery{PageNum: 2, PageSize: 10}, 2, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.q.Num(); got != tt.wantNum {
				t.Errorf("Num() = %d, want %d", got, tt.wantNum)
			}
			if got := tt.q.Size(); got != tt.wantSize {
				t.Errorf("Size() = %d, want %d", got, tt.wantSize)
			}
			if got := tt.q.Offset(); got != tt.wantOffset {
				t.Errorf("Offset() = %d, want %d", got, tt.wantOffset)
			}
		})
	}
}

// TestOrderByMatchesJavaSemantics 对齐 Java 端 PageQuery 的排序用法。
func TestOrderByMatchesJavaSemantics(t *testing.T) {
	tests := []struct {
		name    string
		column  string
		isAsc   string
		wantSQL string
	}{
		{"单列升序", "id", "asc", "ORDER BY `id`"},
		{"多列同向升序", "id,createTime", "asc", "ORDER BY `id`,`create_time`"},
		{"多列同向降序", "id,createTime", "desc", "ORDER BY `id` DESC,`create_time` DESC"},
		{"多列各自方向", "id,createTime", "asc,desc", "ORDER BY `id`,`create_time` DESC"},
		{"兼容 element-plus", "createTime", "descending", "ORDER BY `create_time` DESC"},
		{"带表别名", "u.createTime", "asc", "ORDER BY `u`.`create_time`"},
		{"已是下划线", "create_time", "desc", "ORDER BY `create_time` DESC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := PageQuery{OrderByColumn: tt.column, IsAsc: tt.isAsc}
			order, err := q.OrderBy()
			if err != nil {
				t.Fatalf("OrderBy() 意外报错: %v", err)
			}

			var rows []flagUser
			tx := dryDB(t).Model(&flagUser{}).Order(order).Find(&rows)
			got := sqlOf(t, tx.Statement.DB)
			if !strings.Contains(got, tt.wantSQL) {
				t.Errorf("SQL = %s\n应包含 %s", got, tt.wantSQL)
			}
		})
	}
}

// TestOrderByEmptyReturnsNoColumns 未传排序参数时不产生 ORDER BY。
func TestOrderByEmptyReturnsNoColumns(t *testing.T) {
	for _, q := range []PageQuery{
		{},
		{OrderByColumn: "id"}, // 只给列不给方向
		{IsAsc: "asc"},        // 只给方向不给列
		{OrderByColumn: "  ", IsAsc: " "},
	} {
		order, err := q.OrderBy()
		if err != nil {
			t.Fatalf("OrderBy(%+v) 意外报错: %v", q, err)
		}
		if len(order.Columns) != 0 {
			t.Errorf("OrderBy(%+v) 应无排序列, got %+v", q, order.Columns)
		}
	}
}

// TestOrderByRejectsInjection 注入类参数必须被拒——列名无法参数化，这里是唯一防线。
func TestOrderByRejectsInjection(t *testing.T) {
	bad := []string{
		"id; DROP TABLE sys_user",
		"id) UNION SELECT password FROM sys_user--",
		"(SELECT sleep(5))",
		"id'",
		"updatexml(1,concat(0x7e,user()),1)",
		"id/*x*/",
		"id`",
	}
	for _, col := range bad {
		q := PageQuery{OrderByColumn: col, IsAsc: "asc"}
		if _, err := q.OrderBy(); err == nil {
			t.Errorf("OrderByColumn=%q 应被拒绝", col)
		}
	}
}

// TestOrderByRejectsBadDirection 方向词非法或与列数不匹配时报错。
func TestOrderByRejectsBadDirection(t *testing.T) {
	tests := []struct{ column, isAsc string }{
		{"id", "up"},                         // 方向词拼错
		{"id,createTime", "asc,desc,asc"},    // 方向数多于列数
		{"id,createTime,status", "asc,desc"}, // 方向数既非 1 也不等于列数
	}
	for _, tt := range tests {
		q := PageQuery{OrderByColumn: tt.column, IsAsc: tt.isAsc}
		_, err := q.OrderBy()
		if err == nil {
			t.Errorf("column=%q isAsc=%q 应被拒绝", tt.column, tt.isAsc)
		}
		if !errors.Is(err, ErrInvalidOrderBy) {
			t.Errorf("column=%q 错误应为 ErrInvalidOrderBy, got %v", tt.column, err)
		}
	}
}

func TestToUnderScoreCase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"createTime", "create_time"},
		{"create_time", "create_time"},
		{"id", "id"},
		{"userID", "user_id"},
		{"UserName", "user_name"},
		{"u.createTime", "u.create_time"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := toUnderScoreCase(tt.in); got != tt.want {
			t.Errorf("toUnderScoreCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
