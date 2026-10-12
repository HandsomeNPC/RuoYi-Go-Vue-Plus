package handler

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/excelhelper"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/response"
)

// reportExportErr 把 excelhelper.Export 的失败登进 gin.Context，交由 Recover 中间件渲染。
//
// 行数超上限是用户可干预的（缩小查询范围），按业务异常回友好文案；
// 其余失败多为工作簿构建异常，按系统异常兜底，不把原始技术信息回前端。
func reportExportErr(c *gin.Context, err error) {
	var over *excelhelper.TooManyRowsError
	if errors.As(err, &over) {
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("导出数据量过大，最多 %d 行，请缩小查询范围后重试", over.Limit),
			err.Error()))
		return
	}
	_ = c.Error(err)
}
