package api

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func SelectEmailRecordPage(c *gin.Context) {
	var emailRecordPageDto dto.EmailRecordPageDto
	_ = c.ShouldBindQuery(&emailRecordPageDto)
	if err := validate.Struct(&emailRecordPageDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	emailRecordPageVo, err := service.SelectEmailRecordPage(
		emailRecordPageDto.ToEmail, emailRecordPageDto.State,
		emailRecordPageDto.PageNum, emailRecordPageDto.PageSize)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(emailRecordPageVo, c)
}
