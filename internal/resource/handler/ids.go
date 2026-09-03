package handler

import (
	"fmt"
	"strconv"
	"strings"
)

// parseIDs 解析逗号分隔的主键串。
//
// 任一段非法即整体拒绝，不静默丢弃：那会把「删 3 个」变成「删了 2 个还报成功」。
// system/monitor 的 handler 包各有一份同形实现，跨包取不到，故本包自备。
func parseIDs(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("handler: 非法主键 %q", p)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("handler: 主键为空")
	}
	return ids, nil
}
