// Package service auth 模块业务逻辑层。
//
// in-process 复用 internal/system 的 service(用户/角色/菜单)，直接函数调用，
// 无网络开销。因此 auth 进程需连接同一数据库。
package service
