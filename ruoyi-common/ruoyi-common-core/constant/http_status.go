package constant

// 返回状态码
//
// @author Lion Li
const (
	// HTTPStatusSuccess 操作成功
	HTTPStatusSuccess = 200

	// HTTPStatusCreated 对象创建成功
	HTTPStatusCreated = 201

	// HTTPStatusAccepted 请求已经被接受
	HTTPStatusAccepted = 202

	// HTTPStatusNoContent 操作已经执行成功，但是没有返回数据
	HTTPStatusNoContent = 204

	// HTTPStatusMovedPerm 资源已被移除
	HTTPStatusMovedPerm = 301

	// HTTPStatusSeeOther 重定向
	HTTPStatusSeeOther = 303

	// HTTPStatusNotModified 资源没有被修改
	HTTPStatusNotModified = 304

	// HTTPStatusBadRequest 参数列表错误（缺少，格式不匹配）
	HTTPStatusBadRequest = 400

	// HTTPStatusUnauthorized 未授权
	HTTPStatusUnauthorized = 401

	// HTTPStatusForbidden 访问受限，授权过期
	HTTPStatusForbidden = 403

	// HTTPStatusNotFound 资源，服务未找到
	HTTPStatusNotFound = 404

	// HTTPStatusBadMethod 不允许的http方法
	HTTPStatusBadMethod = 405

	// HTTPStatusConflict 资源冲突，或者资源被锁
	HTTPStatusConflict = 409

	// HTTPStatusUnsupportedType 不支持的数据，媒体类型
	HTTPStatusUnsupportedType = 415

	// HTTPStatusError 系统内部错误
	HTTPStatusError = 500

	// HTTPStatusNotImplemented 接口未实现
	HTTPStatusNotImplemented = 501

	// HTTPStatusWarn 系统警告消息
	HTTPStatusWarn = 601
)
