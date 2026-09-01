package constant

// 推送消息类型，对照 Java org.dromara.common.core.enums.PushTypeEnum。
//
// 只有一个标识串、无附加信息，故按 pkg/enum/README.md 的拆分标准放 constant
// 而非 enum（enum 只收「标识 + 若干附属字段且需按标识反查」的枚举）。
const (
	// PushTypeMessage 通用消息。
	PushTypeMessage = "message"
	// PushTypeNotice 通知公告。
	PushTypeNotice = "notice"
	// PushTypeLLM 大模型消息，不入消息盒子。
	PushTypeLLM = "llm"
	// PushTypeCustom 客户端自定义消息。
	PushTypeCustom = "custom"
)

// 推送消息来源，对照 Java org.dromara.common.core.enums.PushSourceEnum。
const (
	// PushSourceBackend 后端系统消息。
	PushSourceBackend = "backend"
	// PushSourceNotice 通知公告。
	PushSourceNotice = "notice"
	// PushSourceWorkflow 工作流。
	PushSourceWorkflow = "workflow"
	// PushSourceLLM 大模型。
	PushSourceLLM = "llm"
	// PushSourceClient 客户端消息。
	PushSourceClient = "client"
)

// 消息盒子分类，对照 Java SysMessageServiceImpl 的 CATEGORY_* 私有常量。
// 落库进 sys_message.category，前端按它分三个 tab 展示。
const (
	// MessageCategorySystem 系统消息。
	MessageCategorySystem = "system"
	// MessageCategoryNotice 通知公告。
	MessageCategoryNotice = "notice"
	// MessageCategoryWorkflow 工作流消息。
	MessageCategoryWorkflow = "workflow"
)

// MessageGlobalUserIDs 全局广播的接收人标识，落 sys_message.send_user_ids。
// 对照 Java SysMessageServiceImpl.GLOBAL_USER_IDS，所有用户都能查到。
const MessageGlobalUserIDs = "0"

// MessageTopic 跨进程推送的 Redis 订阅主题。
//
// 与 Java MessageConstants.MESSAGE_TOPIC 逐字一致：nginx 后可能挂多个 system
// 实例，且需要与 Java 侧混部时互通，主题名不能改。
const MessageTopic = "global:message"
