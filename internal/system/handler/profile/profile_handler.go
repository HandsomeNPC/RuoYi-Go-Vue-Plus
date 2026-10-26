// Package profile 个人信息 HTTP 接口。
package profile

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	userservice "ruoyi-go-vue-plus/internal/system/service/user"
)

// Profile 获取当前登录用户的个人中心信息。
// 不校验权限码，仅需登录；user 查不到时返回失败。
func Profile(c *gin.Context) {
	userID := loginhelper.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	res, err := userservice.SelectUserProfile(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if res == nil {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// UpdateProfile 修改当前登录用户的个人资料。
func UpdateProfile(c *gin.Context) {
	userID := loginhelper.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	var b bo.SysUserProfileBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	username := loginhelper.GetUsername(c)
	if err := userservice.UpdateUserProfile(c.Request.Context(), userID, &b); err != nil {
		if errors.Is(err, userservice.ErrUserPhoneExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改用户'%s'失败，手机号码已存在", username), ""))
			return
		}
		if errors.Is(err, userservice.ErrUserEmailExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改用户'%s'失败，邮箱账号已存在", username), ""))
			return
		}
		// ErrUserProfileUpdate 已含「修改个人信息异常，请联系管理员」文案，原样上抛。
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// UpdatePwd 重置密码。
// 走 apiencrypt 中间件：前端以 RSA 加密请求体，中间件解密后 handler 拿到明文。
func UpdatePwd(c *gin.Context) {
	userID := loginhelper.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	var b bo.SysUserPasswordBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := userservice.ChangeUserPassword(c.Request.Context(),
		userID, b.OldPassword, b.NewPassword); err != nil {
		if errors.Is(err, userservice.ErrUserPasswordWrong) {
			_ = c.Error(errs.New(response.CodeFail, "修改密码失败，旧密码错误", ""))
			return
		}
		if errors.Is(err, userservice.ErrUserPasswordSame) {
			_ = c.Error(errs.New(response.CodeFail, "新密码不能与旧密码相同", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
