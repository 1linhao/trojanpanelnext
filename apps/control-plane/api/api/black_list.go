package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func SelectBlackListPage(c *gin.Context) {
	var blackListPageDto dto.BlackListPageDto
	_ = c.ShouldBindQuery(&blackListPageDto)
	if err := validate.Struct(&blackListPageDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}

	blackListPageVo, err := service.SelectBlackListPage(blackListPageDto.Ip, blackListPageDto.PageNum, blackListPageDto.PageSize)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(blackListPageVo, c)
}

func DeleteBlackListByIp(c *gin.Context) {
	var blackListDeleteDto dto.BlackListDeleteDto
	_ = c.ShouldBindJSON(&blackListDeleteDto)
	if err := validate.Struct(&blackListDeleteDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}

	if err := service.DeleteBlackListByIp(blackListDeleteDto.Ip); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func CreateBlackList(c *gin.Context) {
	var blackListCreateDto dto.BlackListCreateDto
	_ = c.ShouldBindJSON(&blackListCreateDto)
	if err := validate.Struct(&blackListCreateDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}

	if err := service.CreateBlackList([]string{*blackListCreateDto.Ip}); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}
