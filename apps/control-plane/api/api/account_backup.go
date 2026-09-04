package api

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

// ExportAccount 导出用户
func ExportAccount(c *gin.Context) {
	accountVo := service.GetCurrentAccount(c)
	if err := service.ExportAccount(accountVo.Id, accountVo.Username); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}

// ImportAccount 导入用户
func ImportAccount(c *gin.Context) {
	coverStr, b := c.GetPostForm("cover")
	if !b {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	cover, err := strconv.ParseUint(coverStr, 10, 32)
	file, err := c.FormFile("file")
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	// 文件大小 10MB
	if file.Size > 1024*1024*10 {
		vo.Fail(constant.FileSizeTooBig, c)
		return
	}
	// 文件后缀.json
	if !strings.HasSuffix(file.Filename, ".json") {
		vo.Fail(constant.FileFormatError, c)
		return
	}
	account := service.GetCurrentAccount(c)
	if err := service.ImportAccount(uint(cover), file, account.Id, account.Username); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}

func ExportAccountUnused(c *gin.Context) {
	accountVo := service.GetCurrentAccount(c)
	if err := service.ExportAccountUnused(accountVo.Id, accountVo.Username); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}
