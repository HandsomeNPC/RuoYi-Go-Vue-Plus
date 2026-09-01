package service

import (
	"context"
	"errors"
	"log"
	"strconv"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// ErrNoticeNotFound 通知公告不存在。
var ErrNoticeNotFound = errors.New("service: 通知公告不存在")

// noticeTypeDictType 公告类型字典，用于新增时取类型标签拼推送文案。
const noticeTypeDictType = "sys_notice_type"

// noticeCreateByNoMatch 创建人账号查不到用户时使用的哨兵值。
//
// 不能退化成"不按创建人筛"：那会把全部公告都返回，与用户「只看某人发的公告」
// 的意图相反。用负数保证 create_by 永不命中（雪花 ID 恒为正），
// 等价于 Java `eq(createBy, null)` 查不出任何行的效果。
const noticeCreateByNoMatch int64 = -1

// NoticeService 通知公告业务逻辑。
type NoticeService struct{}

// NoticeSvcApp 包级实例。
var NoticeSvcApp = new(NoticeService)

// QueryPageList 按条件分页查通知公告，并回填创建人名称。
func (s *NoticeService) QueryPageList(ctx context.Context, q bo.SysNoticeQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysNoticeVo], error) {

	createBy, err := s.resolveCreateBy(ctx, q.CreateByName)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysNoticeVo](), err
	}

	res, err := repository.NewNoticeRepository(database.DB()).
		SelectPageList(ctx, q, createBy, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysNoticeVo](), err
	}

	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	rows := vo.Conv.ConvertToSysNoticeVoList(res.Rows)
	if err := s.fillCreateByNames(ctx, rows); err != nil {
		return pkgrepo.EmptyPage[*vo.SysNoticeVo](), err
	}
	return pkgrepo.Page(rows, res.Total), nil
}

// resolveCreateBy 把创建人账号换成用户ID（对齐 Java 先查 userMapper 再 eq createBy）。
// 账号为空表示不按创建人筛；查不到用户时返回必然不匹配的哨兵值。
func (s *NoticeService) resolveCreateBy(ctx context.Context, createByName string) (int64, error) {
	if createByName == "" {
		return 0, nil
	}

	user, err := repository.NewUserRepository(database.DB()).
		SelectByUserName(ctx, createByName)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return noticeCreateByNoMatch, nil
		}
		return 0, err
	}
	return user.UserID, nil
}

// fillCreateByNames 批量回填创建人名称（对齐 VO 上的 @Translation(USER_ID_TO_NAME)）。
//
// 先去重再一次 IN 查库，不逐行单查——列表页 10 行会打 10 次库。
// 走 sys_user_name 缓存组，与 Java @Cacheable(SYS_USER_NAME, key="#userId") 对齐。
func (s *NoticeService) fillCreateByNames(ctx context.Context, rows []*vo.SysNoticeVo) error {
	pending := make([]int64, 0, len(rows))
	// seen 单独记账：names 里存的是"已解析出的名字"，空串可能是真实的空名，
	// 不能用它当"已处理"标记——那样重复 ID 会反复进待查列表，白打 N 次库。
	seen := make(map[int64]struct{}, len(rows))
	names := make(map[int64]string, len(rows))

	for _, row := range rows {
		if row.CreateBy <= 0 {
			continue
		}
		if _, ok := seen[row.CreateBy]; ok {
			continue
		}
		seen[row.CreateBy] = struct{}{}

		var cached string
		if hit, _ := cache.Get(ctx, constant.CacheSysUserName,
			userNameCacheKey(row.CreateBy), &cached); hit {
			names[row.CreateBy] = cached
			continue
		}
		pending = append(pending, row.CreateBy)
	}

	if len(pending) > 0 {
		found, err := repository.NewUserRepository(database.DB()).
			SelectUserNamesByIDs(ctx, pending)
		if err != nil {
			return err
		}
		for id, name := range found {
			names[id] = name
			_ = cache.Put(ctx, constant.CacheSysUserName, userNameCacheKey(id), name,
				constant.CacheTTLSysUserName)
		}
	}

	for _, row := range rows {
		row.CreateByName = names[row.CreateBy]
	}
	return nil
}

// userNameCacheKey 用户账号缓存的 key，与 Java @Cacheable(key = "#userId") 同形。
func userNameCacheKey(userID int64) string {
	return strconv.FormatInt(userID, 10)
}

// QueryByID 按主键查通知公告（对应 Java selectNoticeById），不存在返回 ErrNoticeNotFound。
func (s *NoticeService) QueryByID(ctx context.Context, noticeID int64) (*vo.SysNoticeVo, error) {
	notice, err := repository.NewNoticeRepository(database.DB()).SelectByID(ctx, noticeID)
	if err != nil {
		if errors.Is(err, repository.ErrNoticeNotFound) {
			return nil, ErrNoticeNotFound
		}
		return nil, err
	}

	out := vo.Conv.ConvertToSysNoticeVo(notice)
	if err := s.fillCreateByNames(ctx, []*vo.SysNoticeVo{out}); err != nil {
		return nil, err
	}
	return out, nil
}

// InsertNotice 新增通知公告并向全部在线用户广播（对应 Java insertNotice + Controller 的 publishAll）。
// 插入成功后回填 b.NoticeID。
//
// 广播编排放在 service 而非 handler：Java 把它摊在 Controller 里，但"新增公告要
// 通知全员"是业务规则而非 HTTP 关注点，放这里才挡得住将来其它调用路径。
func (s *NoticeService) InsertNotice(ctx context.Context, b *bo.SysNoticeBo) error {
	if b == nil {
		return errors.New("service: 通知公告入参为空")
	}

	add := bo.Conv.ConvertToSysNotice(b)
	add.NoticeID = snowflake.Next() // notice_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewNoticeRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.NoticeID = add.NoticeID

	s.broadcastNotice(ctx, b)
	return nil
}

// broadcastNotice 把新公告摘要推给全部在线用户。
//
// 失败只记日志不上抛：公告已经落库，前端刷新就能看到，不值得为一次推送失败
// 让新增接口报错——那会诱使用户重复提交，出两条一样的公告。
func (s *NoticeService) broadcastNotice(ctx context.Context, b *bo.SysNoticeBo) {
	label := s.noticeTypeLabel(ctx, b.NoticeType)
	// data 的键名与 Java 侧逐字一致：前端按它们渲染公告弹窗。
	data := map[string]any{
		"noticeType":      b.NoticeType,
		"noticeTypeLabel": label,
		"noticeTitle":     b.NoticeTitle,
		"noticeId":        b.NoticeID,
		"noticeContent":   b.NoticeContent,
		"status":          b.Status,
	}
	payload := dto.NewPushPayloadWithPath(
		constant.PushTypeNotice,
		constant.PushSourceNotice,
		"["+label+"] "+b.NoticeTitle,
		data,
		"/system/notice?noticeId="+strconv.FormatInt(b.NoticeID, 10),
	)

	if err := MessageSvcApp.PublishAll(ctx, payload); err != nil {
		log.Printf("[notice] 公告广播失败(公告已入库，不影响新增): %v", err)
	}
}

// noticeTypeLabel 取公告类型的字典标签（对应 Java dictService.getDictLabel）。
// 查不到时返回原始值，让文案退化成 "[1] 标题" 而非 "[] 标题"。
func (s *NoticeService) noticeTypeLabel(ctx context.Context, noticeType string) string {
	if noticeType == "" {
		return ""
	}
	rows, err := DictTypeSvcApp.SelectDictDataByType(ctx, noticeTypeDictType)
	if err != nil {
		log.Printf("[notice] 查询公告类型字典失败: %v", err)
		return noticeType
	}
	for _, row := range rows {
		if row.DictValue == noticeType {
			return row.DictLabel
		}
	}
	return noticeType
}

// UpdateNotice 修改通知公告（对应 Java updateNotice）。主键不存在返回 ErrNoticeNotFound。
func (s *NoticeService) UpdateNotice(ctx context.Context, b *bo.SysNoticeBo) error {
	if b == nil {
		return errors.New("service: 通知公告入参为空")
	}
	if b.NoticeID <= 0 {
		return errors.New("service: 通知公告主键不能为空")
	}

	repo := repository.NewNoticeRepository(database.DB())
	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"公告不存在"。
	if _, err := repo.SelectByID(ctx, b.NoticeID); err != nil {
		if errors.Is(err, repository.ErrNoticeNotFound) {
			return ErrNoticeNotFound
		}
		return err
	}

	if _, err := repo.UpdateByID(ctx, b.NoticeID, buildNoticeUpdateColumns(b)); err != nil {
		return err
	}
	return nil
}

// buildNoticeUpdateColumns 组装修改通知公告的更新列。
func buildNoticeUpdateColumns(b *bo.SysNoticeBo) map[string]any {
	columns := map[string]any{
		"notice_title":   b.NoticeTitle,
		"notice_content": b.NoticeContent,
		// 一律写入，让前端能把备注/内容清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"remark": b.Remark,
	}
	// 类型与状态缺省即视为不改：漏传字段不该把线上的 '1'/'0' 刷成空串，
	// 那会让公告既不算通知也不算公告。等效于 Java updateById 对 null 字段的跳过。
	if b.NoticeType != "" {
		columns["notice_type"] = b.NoticeType
	}
	if b.Status != "" {
		columns["status"] = b.Status
	}
	return columns
}

// DeleteNoticeByIDs 批量删除通知公告（对应 Java deleteNoticeByIds）。
func (s *NoticeService) DeleteNoticeByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 通知公告主键不能为空")
	}

	affected, err := repository.NewNoticeRepository(database.DB()).DeleteByIDs(ctx, ids)
	if err != nil {
		return err
	}
	// 一行都没删掉说明主键全不存在，对齐 Java toAjax(0) 的失败口径。
	if affected == 0 {
		return ErrNoticeNotFound
	}
	return nil
}
