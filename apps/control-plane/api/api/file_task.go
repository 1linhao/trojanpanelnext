package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

func SelectFileTaskPage(c *gin.Context) {
	var fileTaskPageDto dto.FileTaskPageDto
	_ = c.ShouldBindQuery(&fileTaskPageDto)
	if err := validate.Struct(&fileTaskPageDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	page, err := service.SelectFileTaskPage(fileTaskPageDto.Type, fileTaskPageDto.AccountUsername, fileTaskPageDto.PageNum, fileTaskPageDto.PageSize)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(page, c)
}

func DeleteFileTaskById(c *gin.Context) {
	var requiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindJSON(&requiredIdDto)
	if err := validate.Struct(&requiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.DeleteFileTaskById(requiredIdDto.Id); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

// DownloadFileTask 下载文件任务的文件
func DownloadFileTask(c *gin.Context) {
	var requiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindJSON(&requiredIdDto)
	if err := validate.Struct(&requiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	fileTask, err := service.SelectFileTaskById(requiredIdDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}

	if fileTask == nil || *fileTask.Status != constant.TaskSuccess {
		vo.Fail(constant.FileTaskNotSuccess, c)
		return
	}

	if !util.Exists(*fileTask.Path) {
		vo.Fail(constant.FileNotExist, c)
		return
	}

	// 设置文件类型
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")
	// 设置文件下载名称
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", *fileTask.Name))
	// 将文件框架输出到响应的正文中
	c.File(*fileTask.Path)
}

func DownloadTemplate(c *gin.Context) {
	var templateRequiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindJSON(&templateRequiredIdDto)
	if err := validate.Struct(&templateRequiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")
	if *templateRequiredIdDto.Id == constant.TaskTypeAccountExport {
		c.Header("Content-Disposition", "attachment; filename=AccountTemplate.json")
		c.File(constant.ExportAccountTemplate)
		return
	} else if *templateRequiredIdDto.Id == constant.TaskTypeNodeServerExport {
		c.Header("Content-Disposition", "attachment; filename=NodeServerTemplate.json")
		c.File(constant.ExportNodeServerTemplate)
		return
	}
	vo.Fail(constant.FileNotExist, c)
}
