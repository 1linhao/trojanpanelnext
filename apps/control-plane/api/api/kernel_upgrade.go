package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

func KernelReleases(c *gin.Context) {
	var request dto.KernelReleaseQueryDto
	if c.ShouldBindQuery(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	catalog, err := service.GetKernelReleases(c.Request.Context(), request.Kernel, request.Channel, request.Refresh)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(catalog, c)
}

func KernelInventory(c *gin.Context) {
	var request dto.KernelInventoryQueryDto
	if c.ShouldBindQuery(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	inventory, err := service.GetNodeKernelInventory(util.GetToken(c), request.NodeServerId)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(inventory, c)
}

func CreateKernelTask(c *gin.Context) {
	var request dto.KernelTaskCreateDto
	if c.ShouldBindJSON(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	task, err := service.CreateKernelTask(request, *service.GetCurrentAccount(c), util.GetToken(c))
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(task, c)
}

func SelectKernelTaskPage(c *gin.Context) {
	var request dto.KernelTaskPageDto
	if c.ShouldBindQuery(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	tasks, total, err := service.SelectKernelTaskPage(*request.PageNum, *request.PageSize, request.Status)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(gin.H{
		"tasks": tasks, "pageNum": *request.PageNum,
		"pageSize": *request.PageSize, "total": total,
	}, c)
}

func SelectKernelTaskById(c *gin.Context) {
	var request dto.KernelTaskIdDto
	if c.ShouldBindQuery(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	task, err := service.SelectKernelTask(request.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(task, c)
}

func RetryKernelTask(c *gin.Context) {
	var request dto.KernelTaskRetryDto
	if c.ShouldBindJSON(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.RetryKernelTask(request, util.GetToken(c)); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func ProbeKernelMTLS(c *gin.Context) {
	var request dto.KernelMTLSProbeDto
	if c.ShouldBindJSON(&request) != nil || validate.Struct(&request) != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.ProbeAndEnableNodeServerMTLS(c.Request.Context(), request.NodeServerId, request.ServerName); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}
