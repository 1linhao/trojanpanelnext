package api

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"path/filepath"
	"strings"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

func SelectSystemByName(c *gin.Context) {
	name := constant.SystemName
	systemVo, err := service.SelectSystemByName(&name)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(systemVo, c)
}

func Setting(c *gin.Context) {
	name := constant.SystemName
	systemVo, err := service.SelectSystemByName(&name)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	settingVo := vo.SettingVo{
		RegisterEnable:     systemVo.RegisterEnable,
		RegisterQuota:      systemVo.RegisterQuota,
		RegisterExpireDays: systemVo.RegisterExpireDays,
		TrafficRankEnable:  systemVo.TrafficRankEnable,
		CaptchaEnable:      systemVo.CaptchaEnable,
		EmailEnable:        systemVo.EmailEnable,
		SystemName:         systemVo.SystemName,
	}
	vo.Success(settingVo, c)
}

func UpdateSystemById(c *gin.Context) {
	var systemUpdateDto dto.SystemUpdateDto
	_ = c.ShouldBindJSON(&systemUpdateDto)
	if err := validate.Struct(&systemUpdateDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.UpdateSystemById(systemUpdateDto); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}

func UploadWebFile(c *gin.Context) {
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
	// 文件后缀.zip
	if !strings.HasSuffix(file.Filename, ".zip") {
		vo.Fail(constant.FileFormatError, c)
		return
	}
	// 删除webfile文件夹内的所有文件
	if err := util.RemoveSubFile(constant.WebFilePath); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	// 保存文件
	webFile := fmt.Sprintf("%s/%s", constant.WebFilePath, constant.WebFileName)
	if err := c.SaveUploadedFile(file, webFile); err != nil {
		vo.Fail(constant.FileUploadError, c)
		return
	}
	// 解压webfile
	if err := util.Unzip(webFile, constant.WebFilePath); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}

func UploadLogo(c *gin.Context) {
	// Include a small allowance for multipart headers while bounding the body.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, util.PanelLogoMaxBytes+64*1024)
	file, err := c.FormFile("file")
	if err != nil {
		var limitError *http.MaxBytesError
		if errors.As(err, &limitError) {
			vo.Fail(constant.FileSizeTooBig, c)
		} else {
			vo.Fail(constant.SysError, c)
		}
		return
	}
	// 文件大小 3MB
	if file.Size > util.PanelLogoMaxBytes {
		vo.Fail(constant.FileSizeTooBig, c)
		return
	}
	// 文件后缀.png
	if !strings.EqualFold(filepath.Ext(file.Filename), ".png") {
		vo.Fail(constant.FileFormatError, c)
		return
	}
	source, err := file.Open()
	if err != nil {
		vo.Fail(constant.FileUploadError, c)
		return
	}
	defer source.Close()
	if err := util.SavePanelLogo(source, constant.LogoImagePath); err != nil {
		switch {
		case errors.Is(err, util.ErrPanelLogoFormat):
			vo.Fail(constant.FileFormatError, c)
		case errors.Is(err, util.ErrPanelLogoSize):
			vo.Fail(constant.FileSizeTooBig, c)
		default:
			vo.Fail(constant.FileUploadError, c)
		}
		return
	}
	vo.Success(nil, c)
}

func GetLogo(c *gin.Context) {
	c.Header("Content-Type", "image/png")
	c.File(constant.LogoImagePath)
}
