package dto

import "time"

// TaskAssigneeDTO 任务受让人分页结果。
type TaskAssigneeDTO struct {
	// Total 总大小。
	Total int64 `json:"total"`
	// List 受让人列表。
	List []TaskHandler `json:"list"`
}

// NewTaskAssignee 创建任务受让人分页结果。
func NewTaskAssignee(total int64, list []TaskHandler) *TaskAssigneeDTO {
	return &TaskAssigneeDTO{Total: total, List: list}
}

// TaskHandler 任务受让人明细对象。
type TaskHandler struct {
	// StorageID 主键。
	StorageID string `json:"storageId"`
	// HandlerCode 权限编码。
	HandlerCode string `json:"handlerCode"`
	// HandlerName 权限名称。
	HandlerName string `json:"handlerName"`
	// GroupName 权限分组。
	GroupName string `json:"groupName"`
	// CreateTime 创建时间。
	CreateTime *time.Time `json:"createTime"`
}

// ConvertToHandlerList 将源列表转换为 TaskHandler 列表。
// 各 mapper 为 nil 时对应字段取零值。
func ConvertToHandlerList[T any](
	source []T,
	storageID func(T) string,
	handlerCode func(T) string,
	handlerName func(T) string,
	groupName func(T) string,
	createTime func(T) *time.Time,
) []TaskHandler {
	if len(source) == 0 {
		return nil
	}
	result := make([]TaskHandler, 0, len(source))
	for _, item := range source {
		h := TaskHandler{}
		if storageID != nil {
			h.StorageID = storageID(item)
		}
		if handlerCode != nil {
			h.HandlerCode = handlerCode(item)
		}
		if handlerName != nil {
			h.HandlerName = handlerName(item)
		}
		if groupName != nil {
			h.GroupName = groupName(item)
		}
		if createTime != nil {
			h.CreateTime = createTime(item)
		}
		result = append(result, h)
	}
	return result
}
