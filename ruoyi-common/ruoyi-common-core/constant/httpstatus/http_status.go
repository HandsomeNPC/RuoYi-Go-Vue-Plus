// Package httpstatus 返回状态码
//
// @author Lion Li
package httpstatus

const (
	// Success 操作成功
	Success = 200

	// Created 对象创建成功
	Created = 201

	// Accepted 请求已经被接受
	Accepted = 202

	// NoContent 操作已经执行成功，但是没有返回数据
	NoContent = 204

	// MovedPerm 资源已被移除
	MovedPerm = 301

	// SeeOther 重定向
	SeeOther = 303

	// NotModified 资源没有被修改
	NotModified = 304

	// BadRequest 参数列表错误（缺少，格式不匹配）
	BadRequest = 400

	// Unauthorized 未授权
	Unauthorized = 401

	// Forbidden 访问受限，授权过期
	Forbidden = 403

	// NotFound 资源，服务未找到
	NotFound = 404

	// BadMethod 不允许的http方法
	BadMethod = 405

	// Conflict 资源冲突，或者资源被锁
	Conflict = 409

	// UnsupportedType 不支持的数据，媒体类型
	UnsupportedType = 415

	// Error 系统内部错误
	Error = 500

	// NotImplemented 接口未实现
	NotImplemented = 501

	// Warn 系统警告消息
	Warn = 601
)
