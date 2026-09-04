package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/pkg/constant"
)

// MessageRepository sys_message 数据访问。
type MessageRepository struct {
	db *gorm.DB
}

// NewMessageRepository 构造消息记录 repository。
func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Insert 插入一条消息记录。
// message_id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *MessageRepository) Insert(ctx context.Context, msg *model.SysMessage) error {
	if msg == nil {
		return fmt.Errorf("repository: 消息记录为空")
	}
	if err := r.db.WithContext(ctx).Create(msg).Error; err != nil {
		return fmt.Errorf("repository: 插入消息记录失败: %w", err)
	}
	return nil
}

// SelectBoxList 查某分类下当前用户可见的消息。
//
// 可见 = 全局广播（send_user_ids = '0'）或当前用户在接收人串里。
// since 为创建时间下界，limit 为最大条数，二者对应「仅 30 天内、最多 100 条」的约束。
func (r *MessageRepository) SelectBoxList(ctx context.Context, category string,
	userID int64, since time.Time, limit int) ([]*model.SysMessage, error) {

	if category == "" || userID <= 0 {
		return nil, nil
	}

	// FIND_IN_SET 而非 LIKE '%id%'：后者会让 userId=1 命中 "10,11" 这类接收人串。
	// 整个 OR 必须括起来，否则会与前面的 AND 条件混合成
	// "category=? AND time>=? AND global OR inset"，让别人的消息漏给当前用户。
	db := r.db.WithContext(ctx).Model(&model.SysMessage{}).
		Where("category = ?", category).
		Where("create_time >= ?", since).
		Where("send_user_ids = ? OR FIND_IN_SET(?, send_user_ids)",
			constant.MessageGlobalUserIDs, userID).
		// 主键兜底保证同秒内顺序稳定。
		Order("create_time DESC").
		Order("message_id DESC")
	if limit > 0 {
		db = db.Limit(limit)
	}

	var rows []*model.SysMessage
	if err := db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("repository: 查询消息盒子(%s)失败: %w", category, err)
	}
	return rows, nil
}
