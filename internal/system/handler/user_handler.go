package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// UserApi 用户信息接口（对应 Java SysUserController）。
type UserApi struct{}

// UserApiApp 包级实例。
var UserApiApp = new(UserApi)

// GetInfo 获取当前登录用户信息、角色与权限集合（对照 Java SysUserController.getInfo）。
// 与 Java 一致：不校验权限码，仅需登录；user 查不到时返回失败。
func (a *UserApi) GetInfo(c *gin.Context) {
	loginUser := loginhelper.GetLoginUser(c)
	if loginUser == nil || loginUser.UserID == 0 {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	user, err := systemservice.UserSvcApp.SelectUserByID(c.Request.Context(), loginUser.UserID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if user == nil {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	// permissions/roles 复用登录时存入 token session 的快照，避免每次拉菜单表。
	c.JSON(http.StatusOK, response.Ok(systemvo.UserInfoVo{
		User:        *user,
		Permissions: loginUser.MenuPermission,
		Roles:       loginUser.RolePermission,
	}))
}

// List 分页查询用户列表（对照 Java list）。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *UserApi) List(c *gin.Context) {
	var q bo.SysUserQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}
	res, err := systemservice.UserSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出用户列表为 xlsx 附件（对照 Java export）。
// 走 POST：前端 commonExport 以 form 表单提交筛选条件，用 ShouldBind 同时吃 form body 与 query。
// 业务 handler 中唯一不返回 response.R 的接口，响应体是二进制附件。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *UserApi) Export(c *gin.Context) {
	var q bo.SysUserQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	rows, err := systemservice.UserSvcApp.QueryExportList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "用户数据"); err != nil {
		_ = c.Error(err)
		return
	}
}

// ImportData 导入用户（对照 Java importData）。
// 走 multipart：file 字段为 xlsx，updateSupport 为是否更新已存在用户。
// 返回导入分析文案（成功/失败汇总），失败行不中断整体解析。
func (a *UserApi) ImportData(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "导入文件不能为空", err.Error()))
		return
	}
	// updateSupport 走表单布尔：前端勾选传 "true"。
	updateSupport := c.PostForm("updateSupport") == "true"

	src, err := file.Open()
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "打开导入文件失败", err.Error()))
		return
	}
	defer func() { _ = src.Close() }()

	msg, err := systemservice.UserSvcApp.ImportUsers(c.Request.Context(), src, updateSupport)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(msg))
}

// ImportTemplate 导出用户导入模板（对照 Java importTemplate）。
// 空切片输出表头列定义，供前端下载后填写再上传。
func (a *UserApi) ImportTemplate(c *gin.Context) {
	if err := excel.Export(c, []systemvo.SysUserImportVo{}, "用户数据"); err != nil {
		_ = c.Error(err)
		return
	}
}

// GetInfoByID 按用户编号获取详情 + 角色ID + 岗位 + 可授权角色（对照 Java getInfo(userId)）。
// 根路径（/system/user）不带 userId，仅返回可授权角色列表；/:userId 带编号时回填用户详情。
func (a *UserApi) GetInfoByID(c *gin.Context) {
	var userID int64
	if raw := c.Param("userId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			_ = c.Error(errs.New(response.CodeBadRequest, "用户编号非法", raw))
			return
		}
		userID = id
	}
	currentUserID := loginhelper.GetUserID(c)
	res, err := systemservice.UserSvcApp.GetUserInfoByID(c.Request.Context(), userID, currentUserID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增用户（对照 Java add）。
// 唯一冲突由 service 返回哨兵错误，handler 翻译为 Java 同款文案。
func (a *UserApi) Add(c *gin.Context) {
	var b bo.SysUserBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	if err := systemservice.UserSvcApp.InsertUser(c.Request.Context(), &b); err != nil {
		translateUserWriteErr(c, err, "新增", b.UserName)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改用户（对照 Java edit）。主键校验单独做：SysUserBo 与新增共用，加 binding:required 会卡住新增。
func (a *UserApi) Edit(c *gin.Context) {
	var b bo.SysUserBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	if b.UserID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", ""))
		return
	}
	if err := systemservice.UserSvcApp.UpdateUser(c.Request.Context(), &b); err != nil {
		translateUserWriteErr(c, err, "修改", b.UserName)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 删除用户（对照 Java remove）。当前登录用户不得在删除集合内。
func (a *UserApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("userIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", err.Error()))
		return
	}
	currentUserID := loginhelper.GetUserID(c)
	if err := systemservice.UserSvcApp.DeleteUserByIDs(c.Request.Context(), currentUserID, ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// OptionSelect 按用户ID串/部门取基础信息（对照 Java optionselect）。
// userIds 为空时返回全部启用用户；deptID > 0 时按部门过滤。
func (a *UserApi) OptionSelect(c *gin.Context) {
	var userIds []int64
	if raw := c.Query("userIds"); raw != "" {
		ids, err := parseIDs(raw)
		if err != nil {
			_ = c.Error(errs.New(response.CodeBadRequest, "用户ID串非法", err.Error()))
			return
		}
		userIds = ids
	}
	var deptID int64
	if raw := c.Query("deptId"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			_ = c.Error(errs.New(response.CodeBadRequest, "部门ID非法", raw))
			return
		}
		deptID = id
	}
	res, err := systemservice.UserSvcApp.SelectByIDs(c.Request.Context(), userIds, deptID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// ResetPwd 重置密码（对照 Java resetPwd）。请求体经 ApiEncrypt 解密后到达此处。
func (a *UserApi) ResetPwd(c *gin.Context) {
	var b bo.SysUserBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	if b.UserID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", ""))
		return
	}
	if err := systemservice.UserSvcApp.ResetUserPwdWithCheck(c.Request.Context(), &b); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// ChangeStatus 修改用户状态（对照 Java changeStatus）。
func (a *UserApi) ChangeStatus(c *gin.Context) {
	var b bo.SysUserBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	if b.UserID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", ""))
		return
	}
	if err := systemservice.UserSvcApp.UpdateUserStatusWithCheck(c.Request.Context(), &b); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Unlock 解锁用户（对照 Java unlock）。
func (a *UserApi) Unlock(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || userID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("userId")))
		return
	}
	if err := systemservice.UserSvcApp.Unlock(c.Request.Context(), userID); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// AuthRole 按用户编号获取授权角色（对照 Java authRole）。
func (a *UserApi) AuthRole(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || userID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("userId")))
		return
	}
	res, err := systemservice.UserSvcApp.AuthRole(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// InsertAuthRole 用户授权角色（对照 Java insertAuthRole）。
// userId、roleIds 走表单字段（Java 无 @RequestBody）；roleIds 兼容重复键与逗号串两种形态。
func (a *UserApi) InsertAuthRole(c *gin.Context) {
	userID, err := strconv.ParseInt(c.PostForm("userId"), 10, 64)
	if err != nil || userID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.PostForm("userId")))
		return
	}
	roleIDs, err := parseFormIDArray(c, "roleIds")
	if err != nil {
		_ = c.Error(err)
		return
	}
	if len(roleIDs) == 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色ID不能为空", ""))
		return
	}
	if err := systemservice.UserSvcApp.InsertUserAuth(c.Request.Context(), userID, roleIDs); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// DeptTree 获取用户筛选用的部门树（对照 Java deptTree）。
func (a *UserApi) DeptTree(c *gin.Context) {
	var q bo.SysDeptQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	res, err := systemservice.UserSvcApp.DeptTree(c.Request.Context(), q)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// ListByDept 获取指定部门下的全部用户（对照 Java listByDept）。
func (a *UserApi) ListByDept(c *gin.Context) {
	deptID, err := strconv.ParseInt(c.Param("deptId"), 10, 64)
	if err != nil || deptID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "部门ID不能为空", c.Param("deptId")))
		return
	}
	res, err := systemservice.UserSvcApp.SelectListByDept(c.Request.Context(), deptID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// parseFormIDArray 解析表单字段为 int64 切片，兼容重复键（a=1&a=2）与逗号串（a=1,2）两种形态。
// 任一段非法即整体拒绝——静默丢弃会授权成部分成功。
func parseFormIDArray(c *gin.Context, field string) ([]int64, error) {
	values := c.PostFormArray(field)
	out := make([]int64, 0, len(values))
	for _, s := range values {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, errs.New(response.CodeBadRequest,
					fmt.Sprintf("字段 %s 含非法主键 %q", field, part), "")
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// translateUserWriteErr 翻译新增/修改用户的哨兵错误为 Java 同款文案。
// 其余错误原样上抛由 middleware.Recover 兜底。
func translateUserWriteErr(c *gin.Context, err error, op, userName string) {
	switch {
	case errors.Is(err, systemservice.ErrUserNameExists):
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("%s用户'%s'失败，登录账号已存在", op, userName), ""))
	case errors.Is(err, systemservice.ErrUserPhoneExists):
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("%s用户'%s'失败，手机号码已存在", op, userName), ""))
	case errors.Is(err, systemservice.ErrUserEmailExists):
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("%s用户'%s'失败，邮箱账号已存在", op, userName), ""))
	default:
		_ = c.Error(err)
	}
}
